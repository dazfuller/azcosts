package formats

import (
	"fmt"
	"github.com/dazfuller/azcosts/internal/model"
	"github.com/xuri/excelize/v2"
	"log"
)

const firstCol = "A"

type ExcelFormatter struct {
	outputPath string
}

func MakeExcelFormatter(outputPath string) (ExcelFormatter, error) {
	if err := validateOptions(false, outputPath); err != nil {
		return ExcelFormatter{}, err
	}

	return ExcelFormatter{outputPath: outputPath}, nil
}

func (ef ExcelFormatter) Generate(costs []model.ResourceGroupSummary) error {
	f := excelize.NewFile()
	defer func(f *excelize.File) {
		err := f.Close()
		if err != nil {
			log.Printf("Unable to close Excel workbook")
		}
	}(f)

	_, _ = f.NewSheet("Sheet2")

	subscriptionSummary := generateSubscriptionSummary(costs)
	err := ef.createSubscriptionSummarySheet(subscriptionSummary, f)
	if err != nil {
		return err
	}

	err = ef.createCostsSheet(costs, f)
	if err != nil {
		return err
	}

	if err = f.SaveAs(ef.outputPath); err != nil {
		return fmt.Errorf("an error occured saving the workbook: %s", err.Error())
	}

	return nil
}

func (ef ExcelFormatter) createSubscriptionSummarySheet(subscriptions []model.SubscriptionSummary, f *excelize.File) error {
	sheetName := "Subscriptions"

	err := f.SetSheetName("Sheet1", sheetName)
	if err != nil {
		return err
	}

	headers := []string{
		"Subscription",
	}

	err = ef.addHeaders(f, sheetName, subscriptions[0].Costs, headers, true)
	if err != nil {
		return err
	}

	err = ef.addSubscriptionCostDataToSheet(f, sheetName, subscriptions)
	if err != nil {
		return err
	}

	cols, _ := f.GetCols(sheetName)
	lastCol, _ := excelize.ColumnNumberToName(len(cols))

	err = ef.setSheetFormats(f, sheetName, cols, 1)
	if err != nil {
		return err
	}

	err = ef.addSparkLines(f, sheetName, 1)
	if err != nil {
		return err
	}

	err2 := ef.addTable(f, sheetName, "SubscriptionSummary", len(subscriptions), lastCol)
	if err2 != nil {
		return err2
	}
	return nil
}

func (ef ExcelFormatter) createCostsSheet(costs []model.ResourceGroupSummary, f *excelize.File) error {
	sheetName := "Costs"

	err := f.SetSheetName("Sheet2", sheetName)
	if err != nil {
		return err
	}

	headers := []string{
		"Resource Group",
		"Subscription",
		"Active",
	}

	err = ef.addHeaders(f, sheetName, costs[0].Costs, headers, true)
	if err != nil {
		return err
	}

	err = ef.addCostDataToSheet(f, sheetName, costs)
	if err != nil {
		return err
	}

	cols, _ := f.GetCols(sheetName)
	lastCol, _ := excelize.ColumnNumberToName(len(cols))

	err = ef.setSheetFormats(f, sheetName, cols, 3)
	if err != nil {
		return err
	}

	err = ef.addSparkLines(f, sheetName, 3)
	if err != nil {
		return err
	}

	err2 := ef.addTable(f, sheetName, "CostSummary", len(costs), lastCol)
	if err2 != nil {
		return err2
	}
	return nil
}

func (ef ExcelFormatter) addHeaders(f *excelize.File, sheetName string, billingPeriods []model.BillingPeriodCost, headers []string, includeChange bool) error {
	firstCell, _ := excelize.JoinCellName("A", 1)

	for _, bp := range billingPeriods {
		headers = append(headers, bp.Period)
	}

	headers = append(headers, "Total Cost")

	if includeChange {
		headers = append(headers, "Change")
	}

	err := f.SetSheetRow(sheetName, firstCell, &headers)
	if err != nil {
		return fmt.Errorf("unable to set header row in costs sheet: %v", err)
	}

	return nil
}

func (ef ExcelFormatter) addCostDataToSheet(f *excelize.File, sheetName string, costs []model.ResourceGroupSummary) error {
	for i, entry := range costs {
		rowStart, _ := excelize.JoinCellName("A", i+2)
		row := []interface{}{
			entry.Name,
			entry.SubscriptionName,
			entry.Active,
		}

		for _, cost := range entry.Costs {
			row = append(row, cost.Total)
		}

		row = append(row, entry.TotalCost)

		err := f.SetSheetRow(sheetName, rowStart, &row)
		if err != nil {
			return fmt.Errorf("unable to add data row to costs worksheet: %v", err)
		}
	}
	return nil
}

func (ef ExcelFormatter) addSubscriptionCostDataToSheet(f *excelize.File, sheetName string, costs []model.SubscriptionSummary) error {
	for i, entry := range costs {
		rowStart, _ := excelize.JoinCellName("A", i+2)
		row := []interface{}{
			entry.Name,
		}

		for _, cost := range entry.Costs {
			row = append(row, cost.Total)
		}

		row = append(row, entry.TotalCost)

		err := f.SetSheetRow(sheetName, rowStart, &row)
		if err != nil {
			return fmt.Errorf("unable to add data row to subscriptions worksheet: %v", err)
		}
	}
	return nil
}

func (ef ExcelFormatter) setSheetFormats(f *excelize.File, sheetName string, cols [][]string, fixedCellCount int) error {
	customNumFmt := "#,##0.00;(#,##0.00);-"

	billingStyle, _ := f.NewStyle(&excelize.Style{
		CustomNumFmt: &customNumFmt, Alignment: &excelize.Alignment{
			Horizontal: "right",
		},
	})

	for i := range cols {
		colName, _ := excelize.ColumnNumberToName(i + 1)
		if i < fixedCellCount {
			maxLength := 0
			for _, v := range cols[i] {
				if len(v) > maxLength {
					maxLength = len(v)
				}
			}
			err := f.SetColWidth(sheetName, colName, colName, float64(maxLength)*0.9)
			if err != nil {
				return fmt.Errorf("unable to set column width for column %s: %v", colName, err)
			}
			continue
		}
		err := f.SetColStyle(sheetName, colName, billingStyle)
		if err != nil {
			return fmt.Errorf("unable to set column style for column %s: %v", colName, err)
		}
		err = f.SetColWidth(sheetName, colName, colName, 15)
		if err != nil {
			return fmt.Errorf("unable to set column width for column %s: %v", colName, err)
		}
	}

	lastCol, _ := excelize.ColumnNumberToName(len(cols))
	lastColWidth := (float64)(len(cols)-5) * 3
	if lastColWidth < 8 {
		lastColWidth = 8
	}
	_ = f.SetColWidth(sheetName, lastCol, lastCol, lastColWidth)

	rows, _ := f.GetRows(sheetName)
	for ri := range rows {
		_ = f.SetRowHeight(sheetName, ri+1, 24)
	}

	return nil
}

func (ef ExcelFormatter) addSparkLines(f *excelize.File, sheetName string, fixedCellCount int) error {
	rows, _ := f.GetRows(sheetName)
	cols, _ := f.GetCols(sheetName)
	lastColumn, _ := excelize.ColumnNumberToName(len(cols))
	startDataColumn, _ := excelize.ColumnNumberToName(fixedCellCount + 1)
	lastDataColumn, _ := excelize.ColumnNumberToName(len(cols) - 2)

	var sparkLineLocation []string
	var sparkLineRange []string
	for i := range rows {
		if i < 1 {
			continue
		}

		ri := i + 1

		location, _ := excelize.JoinCellName(lastColumn, ri)
		start, _ := excelize.JoinCellName(startDataColumn, ri)
		end, _ := excelize.JoinCellName(lastDataColumn, ri)

		sparkLineLocation = append(sparkLineLocation, location)
		sparkLineRange = append(sparkLineRange, fmt.Sprintf("%s!%s:%s", sheetName, start, end))
	}

	return f.AddSparkline(sheetName, &excelize.SparklineOptions{
		Location: sparkLineLocation,
		Range:    sparkLineRange,
		Markers:  true,
		Type:     "line",
		Style:    18,
	})
}

func (ef ExcelFormatter) addTable(f *excelize.File, sheetName string, tableName string, rowCount int, lastCol string) error {
	showHeaderRow := true
	showRowStripes := true

	err := f.AddTable(sheetName, &excelize.Table{
		Range:          fmt.Sprintf("%s%d:%s%d", firstCol, 1, lastCol, rowCount+1),
		Name:           tableName,
		StyleName:      "TableStyleMedium9",
		ShowHeaderRow:  &showHeaderRow,
		ShowRowStripes: &showRowStripes,
	})
	if err != nil {
		return fmt.Errorf("unable to add table to costs sheet: %v", err)
	}

	return nil
}
