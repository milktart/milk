package main

import (
  "fmt"
  "os"
  "strings"

  "github.com/milktart/milk/cmd/flights"
  "github.com/milktart/milk/cmd/miles"
  "github.com/milktart/milk/cmd/numbers"
  "github.com/milktart/milk/cmd/update"
  "github.com/milktart/milk/pkg/config"
)

const (
  TOOLNAME = "milk"
  VERSION  = "0.0.14"
)

func printMainMenu() {
  fmt.Printf("%s - A multi-use CLI tool\n\n", TOOLNAME)
  fmt.Println("Usage:")
  fmt.Printf("  %s <command> [options]\n", TOOLNAME)
  fmt.Printf("  %s --help\n\n", TOOLNAME)
  fmt.Println("Commands:")
  fmt.Println("  numbers    Search for special phone numbers by area code and pattern")
  fmt.Println("  miles      Calculate flight distances between locations")
  fmt.Println("  flights    Fetch and display Delta flight bookings")
  fmt.Println("  update     Check for and install the latest version")
  fmt.Println()
  fmt.Printf("Use \"%s <command> --help\" for more information about a command.\n\n", TOOLNAME)
  fmt.Println("Examples:")
  fmt.Printf("  %s numbers -c 212 415 808 -r Canada -p VIP\n", TOOLNAME)
  fmt.Printf("  %s numbers --Canada\n", TOOLNAME)
  fmt.Printf("  %s miles -R SEA TPE\n", TOOLNAME)
  fmt.Printf("  %s miles AUS KL.Z AMS KL.Z HEL XX PRG KL.N AMS KL.Z AUS\n", TOOLNAME)
}


func main() {
  if len(os.Args) < 2 {
    printMainMenu()
    os.Exit(0)
  }

  subcommand := os.Args[1]

  // Handle help/version flags at main level
  if subcommand == "--help" || subcommand == "-h" || subcommand == "help" {
    printMainMenu()
    os.Exit(0)
  }
  if subcommand == "--version" || subcommand == "-v" || subcommand == "version" {
    fmt.Printf("%s v%s\n", TOOLNAME, VERSION)
    os.Exit(0)
  }

  // update is handled before the version check so it doesn't print a notice
  // about itself before running.
  if strings.ToLower(subcommand) == "update" {
    if err := update.Run(VERSION); err != nil {
      fmt.Fprintf(os.Stderr, "Error: %v\n", err)
      os.Exit(1)
    }
    os.Exit(0)
  }

  // For all other subcommands, check for a newer version and notify if found.
  // Uses a short timeout so it never meaningfully delays the user.
  update.CheckAndNotify(VERSION)

  // Route to subcommands
  switch strings.ToLower(subcommand) {
    case "numbers":
      cfg, err := config.LoadWithUserOverride()
      if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
      }
      handler := numbers.NewHandler(cfg)
      if err := handler.Execute(os.Args[2:]); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
      }

    case "miles":
      handler := miles.NewHandler()
      if err := handler.Execute(os.Args[2:]); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
      }

    case "flights":
      handler := flights.NewHandler()
      if err := handler.Execute(os.Args[2:]); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
      }

    default:
      fmt.Fprintf(os.Stderr, "Error: unknown command '%s'\n\n", subcommand)
      printMainMenu()
      os.Exit(1)
  }
}
