package formats

import (
	"fmt"
	"github.com/dazfuller/azcosts/internal/model"
	"github.com/xuri/excelize/v2"
	"log"
)

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

	sheetName := "Costs"

	err := f.SetSheetName("Sheet1", sheetName)
	if err != nil {
		return err
	}
	firstCell, _ := excelize.JoinCellName("A", 1)

	headers := []string{
		"Resource Group",
		"Subscription",
	}

	for _, bp := range costs[0].Costs {
		headers = append(headers, bp.Period)
	}

	headers = append(headers, "Total Cost")

	err = f.SetSheetRow(sheetName, firstCell, &headers)
	if err != nil {
		return fmt.Errorf("unable to set header row in costs sheet: %s", err.Error())
	}

	for i, entry := range costs {
		rowStart, _ := excelize.JoinCellName("A", i+2)
		row := []interface{}{
			entry.Name,
			entry.SubscriptionName,
		}

		for _, cost := range entry.Costs {
			row = append(row, cost.Total)
		}

		row = append(row, entry.TotalCost)

		err = f.SetSheetRow(sheetName, rowStart, &row)
		if err != nil {
			return fmt.Errorf("unable to add data row to costs worksheet: %s", err.Error())
		}
	}

	billingStyle, _ := f.NewStyle(&excelize.Style{
		NumFmt: 4, Alignment: &excelize.Alignment{
			Horizontal: "right",
		},
	})

	cols, _ := f.GetCols(sheetName)
	firstCol := "A"
	lastCol, _ := excelize.ColumnNumberToName(len(cols))

	for i := range cols {
		colName, _ := excelize.ColumnNumberToName(i + 1)
		if i < 2 {
			maxLength := 0
			for _, v := range cols[i] {
				if len(v) > maxLength {
					maxLength = len(v)
				}
			}
			err = f.SetColWidth(sheetName, colName, colName, float64(maxLength)*0.9)
			if err != nil {
				return fmt.Errorf("unable to set column width for column %s: %s", colName, err.Error())
			}
			continue
		}
		err = f.SetColStyle(sheetName, colName, billingStyle)
		if err != nil {
			return fmt.Errorf("unable to set column style for column %s: %s", colName, err.Error())
		}
		err = f.SetColWidth(sheetName, colName, colName, 15)
		if err != nil {
			return fmt.Errorf("unable to set column width for column %s: %s", colName, err.Error())
		}
	}

	showHeaderRow := true
	showRowStripes := true

	err = f.AddTable(sheetName, &excelize.Table{
		Range:          fmt.Sprintf("%s%d:%s%d", firstCol, 1, lastCol, len(costs)+1),
		Name:           "CostSummary",
		StyleName:      "TableStyleMedium9",
		ShowHeaderRow:  &showHeaderRow,
		ShowRowStripes: &showRowStripes,
	})
	if err != nil {
		return fmt.Errorf("unable to add table to costs sheet: %s", err.Error())
	}

	if err := f.SaveAs(ef.outputPath); err != nil {
		return fmt.Errorf("an error occured saving the workbook: %s", err.Error())
	}

	return nil
}
