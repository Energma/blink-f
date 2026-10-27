package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v3"

	"github.com/Energma/blink-f/internal/update"
)

const releaseRepo = "Energma/blink-f"

func updateCommand() *cli.Command {
	return &cli.Command{
		Name:  "update",
		Usage: "Replace this binary with the latest release (rmx included)",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "check",
				Usage: "Only report whether a newer release exists",
			},
		},
		Action: runUpdate,
	}
}

func runUpdate(ctx context.Context, cmd *cli.Command) error {
	client := update.New(releaseRepo)
	rel, err := client.Latest(ctx)
	if err != nil {
		return fmt.Errorf("look up latest release: %w", err)
	}

	if rel.Tag == version {
		fmt.Printf("blink %s is already the latest release\n", version)
		return nil
	}
	fmt.Printf("current: %s\nlatest:  %s\n", version, rel.Tag)
	if cmd.Bool("check") {
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate running binary: %w", err)
	}
	// Follow the symlink so an install like /usr/local/bin/blink -> elsewhere
	// updates the real file, not the link.
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	fmt.Printf("downloading %s for %s...\n", rel.Tag, exe)
	if err := client.Install(ctx, rel, "blink", exe); err != nil {
		return fmt.Errorf("update: %w", err)
	}
	fmt.Printf("updated to %s\n", rel.Tag)
	return nil
}
