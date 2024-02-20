package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/dazfuller/azcosts/internal/model"
	_ "github.com/mattn/go-sqlite3"
	"os"
	"strings"
)

type CostManagementStore struct {
	dbPath string
	db     *sql.DB
}

// initializeDatabase initializes the database by creating the "costs" table if it doesn't exist.
//
// If an error occurs during table creation, the error is returned.
func initializeDatabase(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS costs
    (
        id INTEGER PRIMARY KEY AUTOINCREMENT
        , billing_from DATETIME
        , billing_period TEXT
        , resource_group TEXT
        , subscription_name TEXT
        , subscription_id TEXT
        , cost READ
        , cost_usd REAL
        , currency TEXT
    )`)

	if err != nil {
		return err
	}

	return nil
}

// NewCostManagementStore creates a new instance of CostManagementStore and initializes the SQLite database.
func NewCostManagementStore(dbPath string) (*CostManagementStore, error) {
	// Check if the db path already exists
	_, err := os.Stat(dbPath)
	if !errors.Is(err, os.ErrNotExist) {
		err := os.Remove(dbPath)
		if err != nil {
			return nil, err
		}
	}

	db, err := sql.Open("sqlite3", dbPath)
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

func (cm *CostManagementStore) SaveCosts(costs []model.ResourceGroupCost) error {
	tx, err := cm.db.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`INSERT INTO costs
		(
			billing_from
			, billing_period
			, resource_group
			, subscription_name
			, subscription_id
			, cost
			, cost_usd
			, currency
		)
		VALUES
		(
			?, ?, ?, ?, ?, ?, ?, ?)
		`)
	if err != nil {
		tx.Rollback()
		return err
	}

	for _, cost := range costs {
		_, err := stmt.Exec(cost.BillingPeriod, cost.BillingPeriod.Format("2006-01"), cost.Name, cost.SubscriptionName, cost.SubscriptionId, cost.Cost, cost.CostUSD, cost.Currency)
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

func (cm *CostManagementStore) GenerateSummaryByResourceGroup() ([]model.ResourceGroupSummary, error) {
	billingPeriods, err := cm.GetBillingPeriods()
	if err != nil {
		return nil, err
	}

	queryBuilder := strings.Builder{}
	queryBuilder.WriteString("SELECT resource_group AS `ResourceGroup`, subscription_name AS `Subscription`\n")

	for _, bp := range billingPeriods {
		queryBuilder.WriteString(fmt.Sprintf(", SUM(cost) filter (where billing_period = '%[1]s') AS `%[1]s`\n", bp))
	}

	queryBuilder.WriteString(", SUM(cost) AS `TotalCost`\n")
	queryBuilder.WriteString("FROM costs\n")
	queryBuilder.WriteString("GROUP BY resource_group, subscription_name\n")
	queryBuilder.WriteString("ORDER BY ResourceGroup\n")

	rows, err := cm.db.Query(queryBuilder.String())
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
		for i := 2; i < len(row)-1; i++ {
			groupBillingCosts = append(groupBillingCosts, model.BillingPeriodCost{
				Period: cols[i],
				Total:  costToFloat(row[i]),
			})
		}

		summary = append(summary, model.ResourceGroupSummary{
			Name:             row[0].(string),
			SubscriptionName: row[1].(string),
			Costs:            groupBillingCosts,
			TotalCost:        costToFloat(row[len(row)-1]),
		})
	}

	return summary, nil
}

func (cm *CostManagementStore) GetBillingPeriods() ([]string, error) {
	rows, err := cm.db.Query("SELECT DISTINCT billing_period FROM costs ORDER BY billing_period")
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
