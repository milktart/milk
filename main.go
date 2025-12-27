package main

import (
  "fmt"
  "os"
  "strings"

  "github.com/milktart/milk/cmd/flights"
  "github.com/milktart/milk/cmd/numbers"
  "github.com/milktart/milk/pkg/config"
)

const (
  TOOLNAME = "milk"
  VERSION  = "0.0.11"
)

func printMainMenu() {
  fmt.Printf("%s - A multi-use CLI tool\n\n", TOOLNAME)
  fmt.Println("Usage:")
  fmt.Printf("  %s <command> [options]\n", TOOLNAME)
  fmt.Printf("  %s --help\n\n", TOOLNAME)
  fmt.Println("Commands:")
  fmt.Println("  numbers    Search for special phone numbers by area code and pattern")
  fmt.Println("  flights    Calculate flight distances between locations")
  fmt.Println()
  fmt.Printf("Use \"%s <command> --help\" for more information about a command.\n\n", TOOLNAME)
  fmt.Println("Examples:")
  fmt.Printf("  %s numbers -c 212 415 808 -r Canada -p VIP\n", TOOLNAME)
  fmt.Printf("  %s numbers --Canada\n", TOOLNAME)
  fmt.Printf("  %s flights -R SEA TPE\n", TOOLNAME)
  fmt.Printf("  %s flights AUS KL.Z AMS KL.Z HEL XX PRG KL.N AMS KL.Z AUS\n", TOOLNAME)
}


func main() {
  if len(os.Args) < 2 {
    printMainMenu()
    os.Exit(0)
  }

  subcommand := os.Args[1]

  // Handle help flags at main level
  if subcommand == "--help" || subcommand == "-h" || subcommand == "help" {
    printMainMenu()
    os.Exit(0)
  }

  // Route to subcommands
  switch strings.ToLower(subcommand) {
    case "numbers":
      cfg, err := config.LoadFromBytes()
      if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
      }
      handler := numbers.NewHandler(cfg)
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
