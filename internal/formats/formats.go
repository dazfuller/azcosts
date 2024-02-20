package formats

import (
	"errors"
	"fmt"
	"github.com/dazfuller/azcosts/internal/model"
	"os"
)

type Formatter interface {
	Generate(costs []model.ResourceGroupSummary) error
}

func validateOptions(useStdOut bool, outputPath string) error {
	if !useStdOut && len(outputPath) == 0 {
		return fmt.Errorf("when writing to file and file path must be specified")
	}

	if !useStdOut {
		_, err := os.Stat(outputPath)
		if !errors.Is(err, os.ErrNotExist) {
			err := os.Remove(outputPath)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
