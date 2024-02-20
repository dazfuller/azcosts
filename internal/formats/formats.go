package formats

import "github.com/dazfuller/azcosts/internal/model"

type Formatter interface {
	Generate(costs []model.ResourceGroupSummary) error
}
