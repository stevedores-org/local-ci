package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// DryRunStage represents a single stage in dry-run output
type DryRunStage struct {
	Name     string `json:"name"`
	Command  string `json:"command"`
	WouldRun bool   `json:"would_run"`
	Reason   string `json:"reason"` // "cached", "hash_changed", "disabled", "no_cache_flag"
}

// DryRunReport represents the overall dry-run output
type DryRunReport struct {
	Workspace  string        `json:"workspace"`
	SourceHash string        `json:"source_hash"`
	Stages     []DryRunStage `json:"stages"`
}

// BuildDryRunReport creates a dry-run report for the given stages
func BuildDryRunReport(stages []Stage, cache map[string]string, sourceHash string, noCache bool) DryRunReport {
	workspace, _ := os.Getwd()

	var dryRunStages []DryRunStage
	for _, stage := range stages {
		dryRunStage := DryRunStage{
			Name:    stage.Name,
			Command: strings.Join(stage.Cmd, " "),
		}

		// Determine if stage would run and reason
		if !stage.Enabled {
			dryRunStage.WouldRun = false
			dryRunStage.Reason = "disabled"
		} else if noCache {
			dryRunStage.WouldRun = true
			dryRunStage.Reason = "no_cache_flag"
		} else if cache[stage.Name] == sourceHash {
			dryRunStage.WouldRun = false
			dryRunStage.Reason = "cached"
		} else {
			dryRunStage.WouldRun = true
			dryRunStage.Reason = "hash_changed"
		}

		dryRunStages = append(dryRunStages, dryRunStage)
	}

	return DryRunReport{
		Workspace:  workspace,
		SourceHash: sourceHash,
		Stages:     dryRunStages,
	}
}

// PrintDryRunJSON prints dry-run report in JSON format
func PrintDryRunJSON(report DryRunReport) {
	data, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(data))
}

// PrintDryRunHuman prints dry-run report in human-readable format
func PrintDryRunHuman(report DryRunReport) {
	fmt.Printf("📋 Dry-run report for: %s\n", report.Workspace)
	fmt.Printf("   Source hash: %s\n\n", report.SourceHash)

	fmt.Println("Stages:")
	for _, stage := range report.Stages {
		status := "✗"
		if stage.WouldRun {
			status = "✓"
		}
		fmt.Printf("  %s %s\n", status, stage.Name)
		fmt.Printf("      Command: %s\n", stage.Command)
		fmt.Printf("      Reason: %s\n", stage.Reason)
	}

	// Summary
	wouldRun := 0
	for _, stage := range report.Stages {
		if stage.WouldRun {
			wouldRun++
		}
	}
	fmt.Printf("\n📊 Summary: %d/%d stages would run\n", wouldRun, len(report.Stages))
}
