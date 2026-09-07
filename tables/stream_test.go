package tables

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/bp0lr/dmut/defines"
)

// Captured from the pre-streaming generator. These fingerprints intentionally
// retain its cumulative multi-label rules, including all eight flag combinations.
func TestLegacyResultsPreserved(t *testing.T) {
	fixtures := []struct {
		subdomain   string
		mask, count int
		hash        string
	}{
		{"", 0, 12, "543dc649201444b7999fa92b6bcfd124991a3ef69e9f1ac46629f6bff74dd9f0"},
		{"", 1, 10, "d5d9384355b40ea12b210ee09b1428a0a191ab0f2c5b94d8607b7b01601f2dc1"},
		{"", 2, 2, "2d343b4b5810193d7efe8557d0963c1ad508996356c91ff8350db715b1f714bc"},
		{"", 3, 0, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{"", 4, 12, "543dc649201444b7999fa92b6bcfd124991a3ef69e9f1ac46629f6bff74dd9f0"},
		{"", 5, 10, "d5d9384355b40ea12b210ee09b1428a0a191ab0f2c5b94d8607b7b01601f2dc1"},
		{"", 6, 2, "2d343b4b5810193d7efe8557d0963c1ad508996356c91ff8350db715b1f714bc"},
		{"", 7, 0, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{"test", 0, 32, "c9a744a3b18246f71613be44a7622cfc15f8a9d6533afc30f0f2c792bbfb4ca6"},
		{"test", 1, 28, "bc35632e13ed7119f1c5a7894e8782af15b6e9e4512a8d220e6ea51b1dfbdbd1"},
		{"test", 2, 12, "7fee7e7e14030b52959c889c956b945f08d7d5ecd3d931057699f193771af310"},
		{"test", 3, 8, "b2e275c38ce15a060d3a4cdd8cda906cd67ad9b5c222a75b485b747642712120"},
		{"test", 4, 24, "88bcbf313b8792c62ec279a4eb2ace0edbb01ed26d07d43d3fb1d13840088ad9"},
		{"test", 5, 20, "7103f2581e997e9f98344759a24d43b228d07581c9eb71c3da9da718544c3bf2"},
		{"test", 6, 4, "b51e44d2f87c9a04a9c0049b8f9df6ed1a0c017a124c68e6573d423422c767fd"},
		{"test", 7, 0, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{"a.b", 0, 62, "cac2103bb9b867dcbddb1c6e40174f8fd6eb30617594c1f5d8bf38804f8bbfc7"},
		{"a.b", 1, 56, "72c909ef1186aad236358be553730487bf1c7d814e43ba569713045ea44aca8c"},
		{"a.b", 2, 22, "62ab9c1b31c3e272fe6bf4426fdb26c5a2fd09f265fb993cb410638e1620f7a0"},
		{"a.b", 3, 16, "30c8ca71797cd3ca8803ab6a9fe6e2632761ff0aa40eddc8d82a35ba52c88b75"},
		{"a.b", 4, 46, "e5a487e11d0317ef717f713a881b4b390f11a9d11275e537b09fbe8f82f0b3fe"},
		{"a.b", 5, 40, "82cb1ad48b2c640c317f72c2716aca63a44077b23912dbc168922c86a08b122d"},
		{"a.b", 6, 6, "279581f2a2d77d16760c3bb81e3f85663b3fdb858c658c3ef776b2743581bfc4"},
		{"a.b", 7, 0, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{"qa-1.api.v2", 0, 92, "4e425dde97dd588b9eedd845dbd075c06353719a419f4e4f70d1f5a42f2de784"},
		{"qa-1.api.v2", 1, 84, "896236920ed02bb5afd5fd84af5ff803c0a13a4feb80af6527eeeed82c3387c4"},
		{"qa-1.api.v2", 2, 32, "d3ac081ed49941ae5febe45cdaf14647855edad77e691af9587b403c469ac7fa"},
		{"qa-1.api.v2", 3, 24, "a99d9566cf72ddf6a49bb6e2ac1f330fcd11dae62ff5afe9861438647cf9f06c"},
		{"qa-1.api.v2", 4, 68, "282d894a79805076fd11478fb4194f956ca56f8699313b85682846e0ca86f649"},
		{"qa-1.api.v2", 5, 60, "f00bcde17a128bcd996d8d26549ff410d2a5cdef958ce780124f8d7d2d444f7d"},
		{"qa-1.api.v2", 6, 8, "5fcf5d372711a6aafdf78a1a6c64ad6b0b44f9ccb85aabd310602bb2d30176cd"},
		{"qa-1.api.v2", 7, 0, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
	}
	words := []string{"stage", "", "dev", "stage"}
	for _, fixture := range fixtures {
		t.Run(fmt.Sprintf("%s/%d", fixture.subdomain, fixture.mask), func(t *testing.T) {
			job := defines.DmutJob{Trd: fixture.subdomain, Sld: "example", Tld: "com"}
			flags := defines.PermutationList{AddToDomain: fixture.mask&1 != 0, AddNumbers: fixture.mask&2 != 0, AddSeparator: fixture.mask&4 != 0}
			var streamed []string
			err := GenerateTo(context.Background(), job, func(visit func(string) error) error {
				for _, word := range words {
					if err := visit(word); err != nil {
						return err
					}
				}
				return nil
			}, flags, func(name string) error { streamed = append(streamed, name); return nil })
			if err != nil {
				t.Fatal(err)
			}
			for name, values := range map[string][]string{"streamed": streamed, "collected": GenerateTables(job, words, flags)} {
				slices.Sort(values)
				values = slices.Compact(values)
				hash := fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(values, "\n"))))
				if len(values) != fixture.count || hash != fixture.hash {
					t.Errorf("%s changed: %d records, %s", name, len(values), hash)
				}
			}
		})
	}
}

func TestGenerateToStopsOnErrorAndCancellation(t *testing.T) {
	job := defines.DmutJob{Trd: "a.b", Sld: "example", Tld: "com"}
	words := func(visit func(string) error) error { return visit("stage") }
	want := errors.New("write failed")
	for mask := 0; mask < 7; mask++ {
		flags := defines.PermutationList{AddToDomain: mask&1 != 0, AddNumbers: mask&2 != 0, AddSeparator: mask&4 != 0}
		calls := 0
		err := GenerateTo(context.Background(), job, words, flags, func(string) error { calls++; return want })
		if !errors.Is(err, want) || calls != 1 {
			t.Fatalf("mask %d: %d calls, %v", mask, calls, err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		calls = 0
		err = GenerateTo(ctx, job, words, flags, func(string) error { calls++; cancel(); return nil })
		cancel()
		if !errors.Is(err, context.Canceled) || calls != 1 {
			t.Fatalf("mask %d: %d calls, %v", mask, calls, err)
		}
	}
	err := GenerateTo(context.Background(), job, func(func(string) error) error { return want }, defines.PermutationList{}, func(string) error { return nil })
	if !errors.Is(err, want) {
		t.Fatal(err)
	}
	// Cancellation by the final emit must be reported even when the source
	// returns immediately and there is no next candidate to trigger a check.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err = GenerateTo(ctx, defines.DmutJob{Sld: "example", Tld: "com"}, words,
		defines.PermutationList{AddNumbers: true, AddSeparator: true},
		func(string) error { cancel(); return nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
