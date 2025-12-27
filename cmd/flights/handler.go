package flights

import (
	"flag"
	"fmt"
)

// Handler processes the distance subcommand
type Handler struct {
	FlagSet *flag.FlagSet
}

// NewHandler creates a new Handler for the distance command
func NewHandler() *Handler {
	return &Handler{
		FlagSet: flag.NewFlagSet("flights", flag.ExitOnError),
	}
}

// Execute runs the distance command with the provided arguments
func (h *Handler) Execute(args []string) error {
	roundtripFlag := h.FlagSet.Bool("roundtrip", false, "Calculate round trip distance (return journey)")
	roundtripShortFlag := h.FlagSet.Bool("R", false, "Shorthand for --roundtrip")
	loyaltyFlag := h.FlagSet.String("l", "None", "Loyalty status for bonus miles (DM, PM, GM, SM, or None)")

	h.FlagSet.Usage = func() {
		fmt.Fprintf(h.FlagSet.Output(), "Usage: milk flights [options] <airport pairs>\n\n")
		fmt.Println("Calculate flight distances and airline miles earnings.\n")
		fmt.Println("Airport pairs are specified as three-letter airport codes.")
		fmt.Println("Optionally prefix each pair with airline.fareclass (e.g., KL.Z for KLM Business).\n")
		fmt.Println("Options:")
		h.FlagSet.PrintDefaults()
		fmt.Println("\nLoyalty Status Options:")
		fmt.Println("  DM - Diamond Member (1.2x bonus)")
		fmt.Println("  PM - Platinum Member (0.8x bonus)")
		fmt.Println("  GM - Gold Member (0.6x bonus)")
		fmt.Println("  SM - Silver Member (0.4x bonus)\n")
		fmt.Println("Examples:")
		fmt.Println("  milk flights ATL LAX")
		fmt.Println("  milk flights -l DM ATL AA.Y LAX DL.J LAS")
		fmt.Println("  milk flights --roundtrip -l PM ORD LAX")
		fmt.Println("  milk flights ATL LAX XX LAX ATL			# Use XX to reset airport for new routes")
	}

	if err := h.FlagSet.Parse(args); err != nil {
		return err
	}

	isRoundTrip := *roundtripFlag || *roundtripShortFlag
	loyaltyStatus := *loyaltyFlag

	// Parse positional arguments to extract routes
	routeArgs := h.FlagSet.Args()
	legs, err := parseRoutes(routeArgs)
	if err != nil {
		return err
	}

	if len(legs) == 0 {
		fmt.Println("Error: No routes specified")
		h.FlagSet.Usage()
		return fmt.Errorf("no routes specified")
	}

	// Add return legs if round trip
	if isRoundTrip {
		returnLegs := make([]Leg, len(legs))
		for i, leg := range legs {
			returnLegs[i] = Leg{
				From:         leg.To,
				To:           leg.From,
				AirlineFare:  leg.AirlineFare,
			}
		}
		// Reverse the return legs so they match the reverse of the outbound
		for i, j := 0, len(returnLegs)-1; i < j; i, j = i+1, j-1 {
			returnLegs[i], returnLegs[j] = returnLegs[j], returnLegs[i]
		}
		legs = append(legs, returnLegs...)
	}

	// Calculate and display results
	return calculateAndDisplay(legs, loyaltyStatus)
}

// Leg represents a flight leg with origin, destination, and optional airline fare class
type Leg struct {
	From        string
	To          string
	AirlineFare string
}

func parseRoutes(args []string) ([]Leg, error) {
	var legs []Leg
	var currentAirport string

	for i := 0; i < len(args); i++ {
		arg := args[i]

		// XX resets the current airport
		if arg == "XX" {
			currentAirport = ""
			continue
		}

		// Check if it's an airport code (3 uppercase letters)
		if len(arg) == 3 && isUppercaseLetters(arg) {
			if currentAirport == "" {
				currentAirport = arg
			} else {
				// This forms a complete leg
				airlineFare := ""

				// Check if previous argument is an airline fare class
				// The airline fare comes BEFORE the destination airport
				if i > 0 {
					prevArg := args[i-1]
					if isAirlineFareClass(prevArg) {
						airlineFare = prevArg
					}
				}

				legs = append(legs, Leg{
					From:        currentAirport,
					To:          arg,
					AirlineFare: airlineFare,
				})
				currentAirport = arg
			}
		}
	}

	return legs, nil
}

func isUppercaseLetters(s string) bool {
	if len(s) != 3 {
		return false
	}
	for _, c := range s {
		if c < 'A' || c > 'Z' {
			return false
		}
	}
	return true
}

func isAirlineFareClass(s string) bool {
	// Pattern: 2-4 uppercase letters, dot, 1 uppercase letter
	if len(s) < 4 || len(s) > 6 {
		return false
	}

	parts := make([]byte, 0, len(s))
	for _, c := range s {
		parts = append(parts, byte(c))
	}

	dotIndex := -1
	for i, c := range parts {
		if c == '.' {
			if dotIndex != -1 {
				return false // Multiple dots
			}
			dotIndex = i
		}
	}

	if dotIndex == -1 || dotIndex < 2 || dotIndex > 4 || dotIndex >= len(parts)-1 {
		return false
	}

	// Check airline code (2-4 uppercase letters before dot)
	for i := 0; i < dotIndex; i++ {
		if parts[i] < 'A' || parts[i] > 'Z' {
			return false
		}
	}

	// Check fare class (1 uppercase letter after dot)
	if parts[dotIndex+1] < 'A' || parts[dotIndex+1] > 'Z' {
		return false
	}
	if dotIndex+2 < len(parts) {
		return false // Too many characters after fare class
	}

	return true
}
