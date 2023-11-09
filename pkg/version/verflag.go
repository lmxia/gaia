// copied from clusternet.

package version

import (
	"encoding/json"
	"fmt"
	"os"

	flag "github.com/spf13/pflag"
	apimachineryversion "k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/pkg/version"
	"sigs.k8s.io/yaml"
)

var versionFlag *string

const (
	versionFlagName = "version"
)

type Version struct {
	ProgramName              string `json:"programName"`
	apimachineryversion.Info `json:",inline"`
}

func AddVersionFlag(fs *flag.FlagSet) {
	versionFlag = fs.String(versionFlagName, "", "Print version with format and quit;"+
		" Available options are 'yaml', 'json' and 'short'")
	fs.Lookup(versionFlagName).NoOptDefVal = "short"
}

func PrintAndExitIfRequested(programName string) error {
	curVersion := Version{
		ProgramName: programName,
		Info:        version.Get(),
	}

	switch *versionFlag {
	case "":
		return nil
	case "short":
		fmt.Printf("%s version: %s\n", programName, curVersion.GitVersion)
	case "yaml":
		y, err := yaml.Marshal(&curVersion)
		if err != nil {
			return err
		}
		fmt.Printf("%s\n", string(y))
	case "json":
		y, err := json.MarshalIndent(&curVersion, "", "  ")
		if err != nil {
			return err
		}
		fmt.Printf("%s\n", string(y))
	default:
		return fmt.Errorf("invalid output format %q", *versionFlag)
	}

	os.Exit(0)
	return nil
}
