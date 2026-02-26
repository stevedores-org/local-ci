package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// DryRunStage describes what would happen to a stage during a dry run.
type DryRunStage struct {
	Name     string `json:"name"`
	Command  string `json:"command"`
	WouldRun bool   `json:"would_run"`
	Reason   string `json:"reason"` // "cached", "hash_changed", "disabled", "no_cache"
}

// DryRunReport is the full dry-run output.
type DryRunReport struct {
	DryRun      bool              `json:"dry_run"`
	Workspace   string            `json:"workspace"`
	StageHashes map[string]string `json:"stage_hashes"`
	Stages      []DryRunStage     `json:"stages"`
	ToRun       int               `json:"to_run"`
	Cached      int               `json:"cached"`
	Disabled    int               `json:"disabled"`
}

// BuildDryRunReport computes a dry-run report for the given stages.
func BuildDryRunReport(
	cwd string,
	stageHashes map[string]string,
	allStages map[string]Stage,
	enabledStages []Stage,
	cache map[string]string,
	noCache bool,
) DryRunReport {
	report := DryRunReport{
		DryRun:      true,
		Workspace:   cwd,
		StageHashes: stageHashes,
	}

	// Report on enabled stages
	for _, stage := range enabledStages {
		cmdStr := strings.Join(stage.Cmd, " ")
		stageHash := stageHashes[stage.Name]
		stageCacheKey := stageHash + "|" + cmdStr

		ds := DryRunStage{
			Name:    stage.Name,
			Command: cmdStr,
		}

		if noCache {
			ds.WouldRun = true
			ds.Reason = "no_cache"
			report.ToRun++
		} else if cache[stage.Name] == stageCacheKey {
			ds.WouldRun = false
			ds.Reason = "cached"
			report.Cached++
		} else {
			ds.WouldRun = true
			ds.Reason = "hash_changed"
			report.ToRun++
		}

		report.Stages = append(report.Stages, ds)
	}

	// Report on disabled stages
	for name, stage := range allStages {
		if stage.Enabled {
			continue
		}
		cmdStr := strings.Join(stage.Cmd, " ")
		report.Stages = append(report.Stages, DryRunStage{
			Name:     name,
			Command:  cmdStr,
			WouldRun: false,
			Reason:   "disabled",
		})
		report.Disabled++
	}

	return report
}

// PrintDryRunHuman prints a human-readable dry-run report.
func PrintDryRunHuman(report DryRunReport) {
	fmt.Println("Dry run — no commands will be executed")
	fmt.Println()
	fmt.Printf("  Workspace: %s\n", report.Workspace)
	fmt.Println()
	fmt.Println("  Stages:")

	idx := 1
	for _, s := range report.Stages {
		tag := ""
		switch s.Reason {
		case "cached":
			tag = "[CACHED - would skip]"
		case "hash_changed":
			tag = "[STALE - would run]"
		case "no_cache":
			tag = "[NO CACHE - would run]"
		case "disabled":
			tag = "[DISABLED]"
		}
		fmt.Printf("    %d. %-12s %-40s %s\n", idx, s.Name, s.Command, tag)
		idx++
	}

	fmt.Println()
	fmt.Printf("  Estimated: %d to run, %d cached, %d disabled\n", report.ToRun, report.Cached, report.Disabled)
}

// PrintDryRunJSON prints the dry-run report as JSON.
func PrintDryRunJSON(report DryRunReport) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(report)
}
