package numbers

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/milktart/milk/pkg/config"
	"github.com/milktart/milk/pkg/tui"
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

	// --add <codes|patterns> <value>  (patterns also requires --tier)
	addFlag := h.FlagSet.String("add", "", "Add to codes or patterns (ex. --add codes 917,646  or  --add patterns 'REGEX' --tier vip)")
	tierFlag := h.FlagSet.String("tier", "", "Tier for --add patterns (vip, platinum, notable)")

	// --remove <codes|patterns>  — interactive
	removeFlag := h.FlagSet.String("remove", "", "Interactively remove codes or patterns (ex. --remove codes  or  --remove patterns)")

	// --list <codes|patterns>
	listFlag := h.FlagSet.String("list", "", "List codes or patterns (ex. --list codes  or  --list patterns)")

	// --edit <codes|patterns>
	editFlag := h.FlagSet.String("edit", "", "Open config file in nano (ex. --edit patterns  or  --edit codes)")
	h.FlagSet.StringVar(editFlag, "e", "", "Same as --edit")

	canadaFlag := h.FlagSet.Bool("Canada", false, "Shorthand for -r Canada")
	CANFlag := h.FlagSet.Bool("CAN", false, "Shorthand for -r CAN")
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
		fmt.Println("  milk numbers -c 416 604")
		fmt.Println("  milk numbers --add codes 917,646")
		fmt.Println("  milk numbers --remove codes")
		fmt.Println("  milk numbers --list codes")
		fmt.Println("  milk numbers --add patterns '(\\d\\d)\\1{4}' --tier vip")
		fmt.Println("  milk numbers --remove patterns")
		fmt.Println("  milk numbers --list patterns")
		fmt.Println("  milk numbers --edit patterns")
		fmt.Println("  milk numbers --edit codes")
	}

	if err := h.FlagSet.Parse(args); err != nil {
		return err
	}

	// Config management flags — handled before search logic.
	if *addFlag != "" {
		switch strings.ToLower(*addFlag) {
		case "codes":
			rest := h.FlagSet.Args()
			if len(rest) == 0 {
				return fmt.Errorf("--add codes requires a value (ex. --add codes 917,646)")
			}
			return AddDefaultCodes(util.SplitList(strings.Join(rest, ",")))
		case "patterns":
			rest := h.FlagSet.Args()
			if len(rest) == 0 {
				return fmt.Errorf("--add patterns requires a pattern value")
			}
			tier := strings.ToLower(*tierFlag)
			if tier == "" {
				return fmt.Errorf("--add patterns requires --tier (vip, platinum, or notable)")
			}
			return config.AddPattern(tier, rest[0])
		default:
			return fmt.Errorf("unknown target %q for --add (use codes or patterns)", *addFlag)
		}
	}

	if *removeFlag != "" {
		switch strings.ToLower(*removeFlag) {
		case "codes":
			return RemoveDefaultCodesInteractive(h.cfg.GetRegionCodes("default"))
		case "patterns":
			return removePatterns()
		default:
			return fmt.Errorf("unknown target %q for --remove (use codes or patterns)", *removeFlag)
		}
	}

	if *listFlag != "" {
		switch strings.ToLower(*listFlag) {
		case "codes":
			return ListCodes(h.cfg.GetRegionCodes("default"))
		case "patterns":
			return listPatterns()
		default:
			return fmt.Errorf("unknown target %q for --list (use codes or patterns)", *listFlag)
		}
	}

	if *editFlag != "" {
		switch strings.ToLower(*editFlag) {
		case "patterns":
			return editFile(config.UserPatternsFile(), config.EmbeddedPatternsYAML())
		case "codes":
			return editCodesFile()
		default:
			return fmt.Errorf("unknown target %q for --edit (use patterns or codes)", *editFlag)
		}
	}

	region := *regionFlag
	if *canadaFlag {
		region = "Canada"
	}
	if *CANFlag {
		region = "CAN"
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
		// Prefer user-configured defaults over built-in defaults.
		if userCfg, err := loadNumbersConfig(); err == nil && userCfg != nil && len(userCfg.DefaultCodes) > 0 {
			codes = userCfg.DefaultCodes
		} else {
			codes = h.cfg.GetRegionCodes("default")
			if codes == nil {
				return fmt.Errorf("no area codes specified and default region not found")
			}
		}
	}

	GetNumbersFiltered(codes, patternTypes)
	return nil
}

// listPatterns prints all patterns grouped by tier.
func listPatterns() error {
	entries, err := config.ListPatterns()
	if err != nil {
		return err
	}
	tiers := []string{"vip", "platinum", "notable"}
	for _, tier := range tiers {
		fmt.Printf("\n%s:\n", strings.ToUpper(tier))
		found := false
		for _, e := range entries {
			if e.Tier == tier {
				fmt.Printf("  %s\n", e.Pattern)
				found = true
			}
		}
		if !found {
			fmt.Println("  (none)")
		}
	}
	fmt.Printf("\nConfig file: %s\n", config.UserPatternsFile())
	return nil
}

// removePatterns opens an interactive selector to remove patterns.
func removePatterns() error {
	entries, err := config.ListPatterns()
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Println("No patterns configured.")
		return nil
	}

	items := make([]tui.Item, len(entries))
	for i, e := range entries {
		items[i] = tui.Item{Label: e.Pattern, Group: strings.ToUpper(e.Tier)}
	}

	indices, err := tui.MultiSelect("Select patterns to remove:", items)
	if err != nil {
		return err
	}
	if len(indices) == 0 {
		fmt.Println("Nothing removed.")
		return nil
	}

	if err := config.RemovePatterns(indices, entries); err != nil {
		return err
	}

	var removed []string
	for _, i := range indices {
		removed = append(removed, entries[i].Pattern)
	}
	fmt.Printf("Removed %d pattern(s): %s\n", len(removed), strings.Join(removed, ", "))
	return nil
}

// editFile seeds path from seedData if missing, then opens it in nano.
func editFile(path string, seedData []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("could not create config directory: %w", err)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.WriteFile(path, seedData, 0644); err != nil {
			return fmt.Errorf("could not write config file: %w", err)
		}
	}
	cmd := exec.Command("nano", path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// editCodesFile opens the numbers config JSON in nano, creating it first if needed.
func editCodesFile() error {
	path := numbersConfigFile()
	var seed []byte
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Seed with current built-in defaults so the user has something to edit.
		seed = []byte("{\n  \"default_codes\": []\n}\n")
	}
	return editFile(path, seed)
}
