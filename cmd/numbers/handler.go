package numbers

import (
	"flag"
	"fmt"
	"strings"

	"github.com/milktart/milk/pkg/config"
	"github.com/milktart/milk/pkg/util"
)

// Handler processes the numbers subcommand
type Handler struct {
	FlagSet *flag.FlagSet
	cfg     *config.Config
}

// NewHandler creates a new Handler for the numbers command
func NewHandler(cfg *config.Config) *Handler {
	return &Handler{
		FlagSet: flag.NewFlagSet("numbers", flag.ExitOnError),
		cfg:     cfg,
	}
}

// Execute runs the numbers command with the provided arguments
func (h *Handler) Execute(args []string) error {
	codeFlag := h.FlagSet.String("c", "", "Comma or space separated list of area codes (ex. -c 212,415,808)")
	h.FlagSet.StringVar(codeFlag, "code", "", "Same as -c")

	regionFlag := h.FlagSet.String("r", "", "Region filter (ex. -r Canada)")
	h.FlagSet.StringVar(regionFlag, "region", "", "Same as -r")

	patternFlag := h.FlagSet.String("p", "", "Pattern type(s) to search (ex. -p VIP,platinum)")
	h.FlagSet.StringVar(patternFlag, "pattern", "", "Same as -p")

	canadaFlag := h.FlagSet.Bool("Canada", false, "Shorthand for -r Canada")
	CAFlag := h.FlagSet.Bool("CA", false, "Shorthand for -r CA")
	NYFlag := h.FlagSet.Bool("NY", false, "Shorthand for -r NY")
	NYCFlag := h.FlagSet.Bool("NYC", false, "Shorthand for -r NYC")
	TXFlag := h.FlagSet.Bool("TX", false, "Shorthand for -r TX")

	h.FlagSet.Usage = func() {
		fmt.Fprintf(h.FlagSet.Output(), "Usage: milk numbers [options]\n\n")
		fmt.Println("Search for special phone numbers by area code and pattern.\n")
		fmt.Println("Options:")
		h.FlagSet.PrintDefaults()
		fmt.Println("\nExamples:")
		fmt.Println("  milk numbers -c 212 415 808 -r Canada -p VIP,platinum")
		fmt.Println("  milk numbers --code 212,415,808 --region TX --pattern VIP")
		fmt.Println("  milk numbers --Canada -c 416 604")
	}

	if err := h.FlagSet.Parse(args); err != nil {
		return err
	}

	region := *regionFlag
	if *canadaFlag {
		region = "Canada"
	}
	if *CAFlag {
		region = "CA"
	}
	if *NYFlag {
		region = "NY"
	}
	if *NYCFlag {
		region = "NYC"
	}
	if *TXFlag {
		region = "TX"
	}

	codes := util.SplitList(*codeFlag)
	patternTypes := util.SplitList(*patternFlag)

	for _, arg := range h.FlagSet.Args() {
		if !strings.HasPrefix(arg, "-") && len(arg) <= 5 {
			codes = append(codes, arg)
		}
	}

	if region != "" && len(codes) == 0 {
		if rc := h.cfg.GetRegionCodes(region); rc != nil {
			codes = rc
		}
	}

	if len(codes) == 0 {
		codes = h.cfg.GetRegionCodes("default")
		if codes == nil {
			return fmt.Errorf("no area codes specified and default region not found")
		}
	}

	GetNumbersFiltered(codes, patternTypes)
	return nil
}
