package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ArqonAi/Pixelog/pkg/publish"
	"github.com/ArqonAi/Pixelog/pkg/publish/arweave"
	"github.com/ArqonAi/Pixelog/pkg/publish/ipfs"
)

// handlePublish uploads a .pixe capsule to the durability networks
// selected via --target (ipfs, arweave, or both). Locators are printed
// as JSON unless --plain is set.
//
// Required env:
//
//	IPFS_API_URL              Kubo-compatible /api/v0/add endpoint
//	IPFS_AUTH_HEADER          (optional) "Bearer <jwt>" for Pinata etc.
//	IPFS_GATEWAY_URL          (optional) gateway for Result.URL
//	ARWEAVE_NODE_URL          Arweave HTTP node (https://arweave.net or arlocal)
//	ARWEAVE_WALLET_PATH       Path to Arweave JWK wallet file
//	ARWEAVE_GATEWAY_URL       (optional) gateway URL; defaults to node URL
func handlePublish() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Error: input .pixe file required")
		fmt.Fprintln(os.Stderr, "Usage: pixe publish <file.pixe> [--target ipfs,arweave] [--mime <type>] [--json]")
		os.Exit(1)
	}

	inputPath := os.Args[2]
	targetsArg := "ipfs,arweave"
	mime := "video/mp4"
	asJSON := true

	for i := 3; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--target":
			if i+1 < len(os.Args) {
				targetsArg = os.Args[i+1]
				i++
			}
		case "--mime":
			if i+1 < len(os.Args) {
				mime = os.Args[i+1]
				i++
			}
		case "--plain":
			asJSON = false
		case "--json":
			asJSON = true
		}
	}

	data, err := os.ReadFile(inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", inputPath, err)
		os.Exit(1)
	}

	publishers, err := buildPublishers(strings.Split(targetsArg, ","))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error configuring publishers: %v\n", err)
		os.Exit(1)
	}
	if len(publishers) == 0 {
		fmt.Fprintln(os.Stderr, "Error: no publishers configured (set IPFS_API_URL and/or ARWEAVE_NODE_URL + ARWEAVE_WALLET_PATH)")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	type pubOutcome struct {
		Network string         `json:"network"`
		Result  publish.Result `json:"result,omitempty"`
		Error   string         `json:"error,omitempty"`
	}
	var results []pubOutcome
	for _, p := range publishers {
		res, err := p.Publish(ctx, data, mime)
		if err != nil {
			results = append(results, pubOutcome{Network: p.Network(), Error: err.Error()})
			continue
		}
		results = append(results, pubOutcome{Network: p.Network(), Result: res})
	}

	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(results)
	} else {
		for _, r := range results {
			if r.Error != "" {
				fmt.Printf("%s: ERROR %s\n", r.Network, r.Error)
				continue
			}
			fmt.Printf("%s: %s", r.Network, r.Result.Locator)
			if r.Result.URL != "" {
				fmt.Printf("  (%s)", r.Result.URL)
			}
			fmt.Println()
		}
	}

	// Exit non-zero if any publisher failed.
	for _, r := range results {
		if r.Error != "" {
			os.Exit(2)
		}
	}
}

// buildPublishers constructs the requested publishers from environment
// variables. Unknown or unconfigured targets are skipped silently so
// `--target ipfs,arweave` stays convenient when only one is set up.
func buildPublishers(targets []string) ([]publish.Publisher, error) {
	var out []publish.Publisher
	for _, raw := range targets {
		t := strings.TrimSpace(strings.ToLower(raw))
		switch t {
		case "", "none":
			continue
		case "ipfs":
			api := os.Getenv("IPFS_API_URL")
			if api == "" {
				continue
			}
			pub, err := ipfs.New(ipfs.Config{
				APIURL:     api,
				AuthHeader: os.Getenv("IPFS_AUTH_HEADER"),
				GatewayURL: os.Getenv("IPFS_GATEWAY_URL"),
			})
			if err != nil {
				return nil, fmt.Errorf("ipfs: %w", err)
			}
			out = append(out, pub)
		case "arweave":
			node := os.Getenv("ARWEAVE_NODE_URL")
			wallet := os.Getenv("ARWEAVE_WALLET_PATH")
			if node == "" || wallet == "" {
				continue
			}
			w, err := arweave.LoadWalletFromFile(wallet)
			if err != nil {
				return nil, fmt.Errorf("arweave wallet: %w", err)
			}
			pub, err := arweave.New(arweave.Config{
				NodeURL:    node,
				Wallet:     w,
				GatewayURL: os.Getenv("ARWEAVE_GATEWAY_URL"),
			})
			if err != nil {
				return nil, fmt.Errorf("arweave: %w", err)
			}
			out = append(out, pub)
		default:
			return nil, fmt.Errorf("unknown target %q (want ipfs or arweave)", t)
		}
	}
	return out, nil
}
