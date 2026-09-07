package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/bp0lr/dmut/tables"
	"github.com/bp0lr/dmut/util"
)

func (a *application) preview(ctx context.Context) (err error) {
	job, err := parseDomain(strings.TrimSpace(a.cfg.Domain))
	if err != nil {
		return fmt.Errorf("preview domain: %w", err)
	}
	file, err := os.Open(a.cfg.Dictionary)
	if err != nil {
		return fmt.Errorf("read dictionary: %w", err)
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	// Sample input as well as output: preview must not enumerate a whole job.
	var words []string
	stopSample := errors.New("preview sample collected")
	err = util.ForEachLine(ctx, file, func(word string) error {
		words = append(words, word)
		if len(words) >= a.cfg.PreviewLimit {
			return stopSample
		}
		return nil
	})
	if err != nil && !errors.Is(err, stopSample) {
		return fmt.Errorf("sample dictionary: %w", err)
	}
	if len(words) == 0 {
		return errors.New("dictionary is empty")
	}
	seen := make(map[string]map[string]bool)
	count := 0
	writer := tabwriter.NewWriter(a.out, 0, 0, 2, ' ', 0)
	if a.cfg.Explain {
		if _, err := fmt.Fprintln(writer, "NAME\tRULE\tWORD"); err != nil {
			return err
		}
	}
	err = tables.GenerateExplainedTo(ctx, job, func(visit func(string) error) error {
		for _, word := range words {
			if err := visit(word); err != nil {
				return err
			}
		}
		return nil
	}, a.cfg.Permutations, func(result tables.GeneratedName) error {
		if seen[result.Rule] == nil {
			seen[result.Rule] = make(map[string]bool)
		}
		examples := seen[result.Rule]
		if len(examples) >= a.cfg.PreviewLimit || examples[result.Name] {
			return nil
		}
		examples[result.Name] = true
		count++
		if a.cfg.Explain {
			word := "-"
			if result.Word != "" {
				word = fmt.Sprintf("%q", result.Word)
			}
			_, err := fmt.Fprintf(writer, "%s\t%s\t%s\n", result.Name, result.Rule, word)
			return err
		}
		_, err := fmt.Fprintln(a.out, result.Name)
		return err
	})
	if err != nil {
		return err
	}
	if err := writer.Flush(); err != nil {
		return err
	}
	_, err = fmt.Fprintf(a.diag, "Offline preview: %d examples from the first %d dictionary entries; at most %d per rule. This is a sample, not a total count.\n", count, len(words), a.cfg.PreviewLimit)
	return errors.Join(err, ctx.Err())
}
