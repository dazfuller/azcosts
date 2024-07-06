package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/dazfuller/azcosts/internal/model"
	_ "modernc.org/sqlite"
	"os"
	"slices"
	"strings"
	"time"
)

const dbVersion = 1

type CostManagementStore struct {
	dbPath string
	db     *sql.DB
}

func getDatabaseVersion(db *sql.DB) (int, error) {
	row := db.QueryRow("PRAGMA user_version")

	var userVersion int
	err := row.Scan(&userVersion)
	if err != nil {
		return 0, err
	}

	return userVersion, nil
}

func updateDbVersion1(db *sql.DB) error {
	_, err := db.Exec(`ALTER TABLE costs ADD resource_group_status TEXT DEFAULT 'inactive';

	PRAGMA user_version = 1;`)

	return err
}

// initializeDatabase initializes the database by creating the "costs" table if it doesn't exist.
//
// If an error occurs during table creation, the error is returned.
func initializeDatabase(db *sql.DB) error {
	ver, err := getDatabaseVersion(db)
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS costs
    (
        id INTEGER PRIMARY KEY AUTOINCREMENT
        , billing_from DATETIME
        , billing_period TEXT
        , resource_group TEXT
        , resource_group_status TEXT
        , subscription_name TEXT
        , subscription_id TEXT
        , cost READ
        , cost_usd REAL
        , currency TEXT
    );`)
	if err != nil {
		return err
	}

	_, err = db.Exec(fmt.Sprintf("PRAGMA user_version = %d", dbVersion))
	if err != nil {
		return err
	}

	if ver < dbVersion {
		err = updateDbVersion1(db)
		if err != nil {
			return err
		}
	}

	return nil
}

// NewCostManagementStore creates a new instance of CostManagementStore and initializes the SQLite database.
func NewCostManagementStore(dbPath string, truncate bool) (*CostManagementStore, error) {
	if truncate {
		// Check if the db path already exists
		_, err := os.Stat(dbPath)
		if !errors.Is(err, os.ErrNotExist) {
			err := os.Remove(dbPath)
			if err != nil {
				return nil, err
			}
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	if err = initializeDatabase(db); err != nil {
		return nil, err
	}

	return &CostManagementStore{
		dbPath: dbPath,
		db:     db,
	}, nil
}

// Close closes the database connection.
// If an error occurs while closing the connection, the error is returned.
func (cm *CostManagementStore) Close() error {
	if err := cm.db.Close(); err != nil {
		return err
	}
	return nil
}

func (cm *CostManagementStore) SaveCosts(costs []model.ResourceGroupCost, currentResourceGroups []model.ResourceGroup) error {
	tx, err := cm.db.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`INSERT INTO costs
		(
			billing_from
			, billing_period
			, resource_group
			, resource_group_status
			, subscription_name
			, subscription_id
			, cost
			, cost_usd
			, currency
		)
		VALUES
		(
			?, ?, ?, ?, ?, ?, ?, ?, ?)
		`)
	if err != nil {
		tx.Rollback()
		return err
	}

	for _, cost := range costs {
		status := "inactive"
		if slices.ContainsFunc(currentResourceGroups, func(rg model.ResourceGroup) bool {
			return cost.Name == rg.Name
		}) {
			status = "active"
		}

		_, err := stmt.Exec(
			cost.BillingPeriod,
			cost.BillingPeriod.Format("2006-01"),
			cost.Name,
			status,
			cost.SubscriptionName,
			cost.SubscriptionId,
			cost.Cost,
			cost.CostUSD,
			cost.Currency)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		return err
	}

	return nil
}

func (cm *CostManagementStore) createSummaryView(billingPeriods []string) error {
	queryBuilder := strings.Builder{}
	queryBuilder.WriteString("DROP VIEW IF EXISTS vw_cost_summary;")
	queryBuilder.WriteString("CREATE VIEW vw_cost_summary AS\n")
	queryBuilder.WriteString("SELECT resource_group AS `ResourceGroup`, subscription_name AS `Subscription`\n")
	queryBuilder.WriteString("    , CASE WHEN current_status = 'active' THEN 1 ELSE 0 END AS 'Active'\n")

	for _, bp := range billingPeriods {
		queryBuilder.WriteString(fmt.Sprintf(", SUM(cost) filter (where billing_period = '%[1]s') AS `%[1]s`\n", bp))
	}

	queryBuilder.WriteString(", SUM(cost) AS `TotalCost`\n")
	queryBuilder.WriteString("FROM (\n")
	queryBuilder.WriteString("    SELECT resource_group, subscription_id, subscription_name, resource_group_status, cost, billing_period\n")
	queryBuilder.WriteString("           , LAST_VALUE(resource_group_status) OVER (PARTITION BY subscription_id, resource_group ORDER BY billing_from RANGE BETWEEN UNBOUNDED PRECEDING AND UNBOUNDED FOLLOWING) AS `current_status`\n")
	queryBuilder.WriteString("    FROM costs\n")
	queryBuilder.WriteString("    WHERE billing_period IN (")

	for i, bp := range billingPeriods {
		if i > 0 {
			queryBuilder.WriteString(", ")
		}
		queryBuilder.WriteString(fmt.Sprintf("'%s'", bp))
	}

	queryBuilder.WriteString(")\n")
	queryBuilder.WriteString(")\n")
	queryBuilder.WriteString("GROUP BY resource_group, subscription_name;\n")

	_, err := cm.db.Exec(queryBuilder.String())
	return err
}

func (cm *CostManagementStore) GenerateSummaryByResourceGroup(months int) ([]model.ResourceGroupSummary, error) {
	billingPeriods, err := cm.GetAllBillingPeriods(months)
	if err != nil {
		return nil, err
	}

	err = cm.createSummaryView(billingPeriods)
	if err != nil {
		return nil, err
	}
	rows, err := cm.db.Query("SELECT * FROM vw_cost_summary ORDER BY ResourceGroup")
	if err != nil {
		return nil, err
	}

	cols, _ := rows.Columns()
	row := make([]any, len(cols))
	rowPtr := make([]any, len(cols))
	for i := range row {
		rowPtr[i] = &row[i]
	}

	var summary []model.ResourceGroupSummary
	for rows.Next() {
		_ = rows.Scan(rowPtr...)
		groupBillingCosts := make([]model.BillingPeriodCost, 0, len(billingPeriods))
		for i := 3; i < len(row)-1; i++ {
			groupBillingCosts = append(groupBillingCosts, model.BillingPeriodCost{
				Period: cols[i],
				Total:  costToFloat(row[i]),
			})
		}

		summary = append(summary, model.ResourceGroupSummary{
			Name:             row[0].(string),
			SubscriptionName: row[1].(string),
			Active:           row[2].(int64) == 1,
			Costs:            groupBillingCosts,
			TotalCost:        costToFloat(row[len(row)-1]),
		})
	}

	if summary == nil {
		return nil, fmt.Errorf("no cost data has yet been collected to report on")
	}

	return summary, nil
}

func (cm *CostManagementStore) GetAllBillingPeriods(months int) ([]string, error) {
	fromDate := time.Now().UTC().AddDate(0, months*-1, 0)
	fromDate = fromDate.AddDate(0, 0, -fromDate.Day()+1).Add(time.Minute)
	rows, err := cm.db.Query("SELECT DISTINCT billing_period FROM costs WHERE billing_from > ? ORDER BY billing_period", fromDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var billingPeriods []string
	for rows.Next() {
		var billingPeriod string
		err := rows.Scan(&billingPeriod)
		if err != nil {
			return nil, err
		}
		billingPeriods = append(billingPeriods, billingPeriod)
	}

	return billingPeriods, nil
}

func (cm *CostManagementStore) GetSubscriptionBillingPeriods(subscriptionId string) ([]string, error) {
	rows, err := cm.db.Query("SELECT DISTINCT billing_period FROM costs WHERE subscription_id = ? ORDER BY billing_period", subscriptionId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var billingPeriods []string
	for rows.Next() {
		var billingPeriod string
		err := rows.Scan(&billingPeriod)
		if err != nil {
			return nil, err
		}
		billingPeriods = append(billingPeriods, billingPeriod)
	}

	return billingPeriods, nil
}

func (cm *CostManagementStore) GetCollectionSummary() ([]model.CollectionSummary, error) {
	rows, err := cm.db.Query(`
		SELECT
			subscription_name
			, subscription_id
			, billing_from
		FROM
			costs
		GROUP BY
			subscription_name
			, subscription_id
			, billing_from
		ORDER BY
			subscription_name
			, billing_from DESC`)
	if err != nil {
		return nil, err
	}

	var collectionSummaries []model.CollectionSummary
	for rows.Next() {
		var summary model.CollectionSummary
		err := rows.Scan(&summary.SubscriptionName, &summary.SubscriptionId, &summary.BillingPeriod)
		if err != nil {
			return nil, err
		}
		collectionSummaries = append(collectionSummaries, summary)
	}

	return collectionSummaries, nil
}

func (cm *CostManagementStore) DeleteSubscriptionBillingPeriod(subscriptionId string, billingPeriod string) error {
	_, err := cm.db.Exec("DELETE FROM costs WHERE subscription_id = ? AND billing_period = ?", subscriptionId, billingPeriod)
	if err != nil {
		return err
	}
	return nil
}

func (cm *CostManagementStore) ListCollectedSubscriptions() ([]model.Subscription, error) {
	rows, err := cm.db.Query("SELECT DISTINCT subscription_id, subscription_name FROM costs ORDER BY subscription_name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subscriptions []model.Subscription
	for rows.Next() {
		var subscription model.Subscription
		err := rows.Scan(&subscription.Id, &subscription.Name)
		if err != nil {
			return nil, err
		}
		subscriptions = append(subscriptions, subscription)
	}

	return subscriptions, nil
}

func costToFloat(value interface{}) float64 {
	switch value.(type) {
	case int8:
		return float64(value.(int8))
	case int16:
		return float64(value.(int16))
	case int32:
		return float64(value.(int32))
	case int64:
		return float64(value.(int64))
	case float32:
		return float64(value.(float32))
	case float64:
		return value.(float64)
	default:
		return 0
	}
}
