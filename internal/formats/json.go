package formats

import (
	"encoding/json"
	"github.com/dazfuller/azcosts/internal/model"
	"os"
	"time"
)

type report struct {
	Generated          time.Time                    `json:"generated"`
	ResourceGroupCount int                          `json:"resourceGroupCount"`
	TotalCost          float64                      `json:"totalCost"`
	Subscriptions      []model.SubscriptionSummary  `json:"subscriptions"`
	ResourceGroups     []model.ResourceGroupSummary `json:"resourceGroups"`
}

type JsonFormatter struct {
	useStdOut  bool
	outputPath string
}

func MakeJsonFormatter(useStdOut bool, outputPath string) (JsonFormatter, error) {
	if err := validateOptions(useStdOut, outputPath); err != nil {
		return JsonFormatter{}, err
	}

	return JsonFormatter{useStdOut: useStdOut, outputPath: outputPath}, nil
}

func (jf JsonFormatter) Generate(costs []model.ResourceGroupSummary) error {
	totalCost := float64(0)
	for i := range costs {
		totalCost += costs[i].TotalCost
	}

	subscriptionSummary := generateSubscriptionSummary(costs)

	report := report{
		Generated:          time.Now().UTC(),
		ResourceGroupCount: len(costs),
		TotalCost:          totalCost,
		Subscriptions:      subscriptionSummary,
		ResourceGroups:     costs,
	}

	if jf.useStdOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			return err
		}

		return nil
	}

	file, err := os.Create(jf.outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		return err
	}

	return nil
}
