// Package tables implements the original name generation rules.
package tables

import (
	"context"
	"strconv"
	"strings"

	def "github.com/bp0lr/dmut/defines"
	"github.com/bp0lr/dmut/util"
)

// GenerateTables retains the collecting API used by the resolution workflow.
func GenerateTables(job def.DmutJob, alterations []string, pList def.PermutationList) []string {
	var res []string
	if !pList.AddToDomain {
		AddToDomain(job, alterations, &res)
	}
	if !pList.AddNumbers {
		AddNumbers(job, &res)
	}
	if !pList.AddSeparator {
		AddSeparator(job, alterations, &res)
	}
	return util.RemoveDuplicatesSlice(res)
}

// GenerateTo emits the legacy names individually without keeping the dictionary
// or generated results. words visits the dictionary once. Output can contain
// duplicates; offline callers deduplicate on disk, not with an unbounded map.
func GenerateTo(ctx context.Context, job def.DmutJob, words func(func(string) error) error, pList def.PermutationList, emit func(string) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	yield := func(name string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		return emit(name)
	}
	if !pList.AddNumbers {
		if err := addNumbers(job, yield); err != nil {
			return err
		}
	}
	if pList.AddToDomain && pList.AddSeparator {
		return ctx.Err()
	}
	if err := words(func(word string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !pList.AddToDomain {
			if err := addWordToDomain(job, word, yield); err != nil {
				return err
			}
		}
		if !pList.AddSeparator {
			return addWordSeparator(job, word, yield)
		}
		return nil
	}); err != nil {
		return err
	}
	return ctx.Err()
}

// AddToDomain inserts dictionary words into the domain.
func AddToDomain(job def.DmutJob, alterations []string, res *[]string) {
	for _, alt := range alterations {
		_ = addWordToDomain(job, alt, collect(res))
	}
}

func collect(res *[]string) func(string) error {
	return func(name string) error {
		*res = append(*res, name)
		return nil
	}
}

func addWordToDomain(job def.DmutJob, alt string, emit func(string) error) error {
	if alt == "" {
		return nil
	}
	parts := strings.Split(job.Trd, ".")
	if parts[0] == "" {
		return emit(alt + "." + job.Sld + "." + job.Tld)
	}
	for i := 0; i <= len(parts); i++ {
		value := util.Insert(parts, i, alt)
		if err := emit(strings.Join(value, ".") + "." + job.Sld + "." + job.Tld); err != nil {
			return err
		}
	}
	return nil
}

// AddNumbers applies the legacy numeric additions.
func AddNumbers(job def.DmutJob, res *[]string) {
	_ = addNumbers(job, collect(res))
}

func addNumbers(job def.DmutJob, emit func(string) error) error {
	for index := 0; index < 10; index++ {
		parts := strings.Split(job.Trd, ".")
		number := strconv.Itoa(index)
		if parts[0] == "" {
			if err := emit(number + "." + job.Sld + "." + job.Tld); err != nil {
				return err
			}
			continue
		}
		// Keep the cumulative changes to earlier labels made by the legacy rule.
		for i := range parts {
			clean := parts[i]
			parts[i] = clean + "-" + number
			if err := emit(strings.Join(parts, ".") + "." + job.Sld + "." + job.Tld); err != nil {
				return err
			}
			parts[i] = clean + number
			if err := emit(strings.Join(parts, ".") + "." + job.Sld + "." + job.Tld); err != nil {
				return err
			}
		}
	}
	return nil
}

// AddSeparator applies the legacy concatenation rules.
func AddSeparator(job def.DmutJob, alterations []string, res *[]string) {
	for _, alt := range alterations {
		_ = addWordSeparator(job, alt, collect(res))
	}
}

func addWordSeparator(job def.DmutJob, alt string, emit func(string) error) error {
	if alt == "" {
		return nil
	}
	parts := strings.Split(job.Trd, ".")
	if parts[0] == "" {
		return nil
	}
	// Keep the cumulative changes to earlier labels made by the legacy rule.
	for i := range parts {
		clean := parts[i]
		for _, value := range []string{clean + "-" + alt, alt + "-" + clean, clean + alt, alt + clean} {
			parts[i] = value
			if err := emit(strings.Join(parts, ".") + "." + job.Sld + "." + job.Tld); err != nil {
				return err
			}
		}
	}
	return nil
}
