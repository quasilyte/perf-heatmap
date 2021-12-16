package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"time"

	"github.com/cespare/subcmd"
	"github.com/google/pprof/profile"
	"github.com/quasilyte/perf-heatmap/heatmap"
)

func main() {
	cmds := []subcmd.Command{
		{
			Name:        "json",
			Description: "create a json index file for a given profile",
			Do:          jsonMain,
		},
		{
			Name:        "stat",
			Description: "try to build an index from a profile and print its stats",
			Do:          statMain,
		},
	}

	subcmd.Run(cmds)
}

func statMain(args []string) {
	if err := cmdStat(args); err != nil {
		log.Fatalf("perf-heatmap stat: error: %v", err)
	}
}

func cmdStat(args []string) error {
	config := heatmap.IndexConfig{}
	fs := flag.NewFlagSet("perf-heatmap stat", flag.ExitOnError)
	fs.Float64Var(&config.Threshold, "threshold", 0.5, `take this % of top records`)
	flagFilename := fs.String("filename", `.*`, `stat only files that match this regex`)
	fs.Parse(args)

	argv := fs.Args()
	if len(argv) != 1 {
		return errors.New("expected exactly 1 positional arg: profile filename")
	}
	profileFilename := argv[0]

	filenameRE, err := regexp.Compile(*flagFilename)
	if err != nil {
		return fmt.Errorf("compile -filename regexp: %w", err)
	}

	index, err := parseProfile(profileFilename, config)
	if err != nil {
		return err
	}

	for _, filename := range index.CollectFilenames() {
		if !filenameRE.MatchString(filename) {
			continue
		}
		i := 0
		fmt.Printf("%s:\n", filename)
		index.InspectFile(filename, func(s heatmap.LineStats) {
			fmt.Printf("[%3d] line %4d: %6.2fs L=%d G=%d\n", i, s.LineNum, time.Duration(s.Value).Seconds(), s.HeatLevel, s.GlobalHeatLevel)
			i++
		})
	}

	return nil
}

func jsonMain(args []string) {
	if err := cmdJson(args); err != nil {
		log.Fatalf("perf-heatmap json: error: %v", err)
	}
}

func cmdJson(args []string) error {
	config := heatmap.IndexConfig{}
	fs := flag.NewFlagSet("perf-heatmap stat", flag.ExitOnError)
	flagValueFormat := fs.String("value-format", "cpu/microseconds",
		`export to this value format`)
	fs.Float64Var(&config.Threshold, "threshold", 0.5,
		`take this % of top records`)
	fs.Parse(args)

	var valueMultiplier float64
	switch *flagValueFormat {
	case "cpu/nanoseconds":
		valueMultiplier = 1.0
	case "cpu/microseconds":
		valueMultiplier = 0.0001
	case "cpu/milliseconds":
		valueMultiplier = 0.00000001
	default:
		return fmt.Errorf("unexpected value format: %s", *flagValueFormat)
	}

	argv := fs.Args()
	if len(argv) != 1 {
		return errors.New("expected exactly 1 positional arg: profile filename")
	}
	profileFilename := argv[0]

	index, err := parseProfile(profileFilename, config)
	if err != nil {
		return err
	}

	result := &jsonRootIndex{}

	allFilenames := index.CollectFilenames()

	for _, filename := range allFilenames {
		f := &jsonFileIndex{Name: filename}
		result.Files = append(result.Files, f)
		index.InspectFile(filename, func(stats heatmap.LineStats) {
			if stats.HeatLevel == 0 {
				return
			}
			value := int(stats.Value)
			if valueMultiplier != 1.0 {
				value = int(float64(value) * valueMultiplier)
			}
			if value == 0 {
				return
			}
			f.Lines = append(f.Lines, jsonLine{
				Num:             stats.LineNum,
				HeatLevel:       stats.HeatLevel,
				GlobalHeatLevel: stats.GlobalHeatLevel,
				Value:           value,
			})
		})
	}

	writeJSON(os.Stdout, result)

	return nil
}

func writeJSON(w io.Writer, root *jsonRootIndex) {
	fmt.Fprintf(w, "{\n")
	fmt.Fprintf(w, "\t\"files\": [\n")
	for i, f := range root.Files {
		fmt.Fprintf(w, "\t\t{\n")
		fmt.Fprintf(w, "\t\t\t\"name\": %q,\n", f.Name)
		fmt.Fprintf(w, "\t\t\t\"lines\": [\n")
		for i, l := range f.Lines {
			fmt.Fprintf(w, "\t\t\t\t[%d, %d, %d, %d]", l.Num, l.HeatLevel, l.GlobalHeatLevel, l.Value)
			if i != len(f.Lines)-1 {
				fmt.Fprintf(w, ",")
			}
			fmt.Fprintf(w, "\n")
		}
		fmt.Fprintf(w, "\t\t\t]\n")
		fmt.Fprintf(w, "\t\t}")
		if i != len(root.Files)-1 {
			fmt.Fprintf(w, ",")
		}
		fmt.Fprintf(w, "\n")
	}
	fmt.Fprintf(w, "\t]\n")
	fmt.Fprintf(w, "}\n")
}

type jsonRootIndex struct {
	Files []*jsonFileIndex `json:"files"`
}

type jsonFileIndex struct {
	Name  string
	Lines []jsonLine
}

type jsonLine struct {
	Num             int
	HeatLevel       int
	GlobalHeatLevel int
	Value           int
}

func parseProfile(profileFilename string, config heatmap.IndexConfig) (*heatmap.Index, error) {
	data, err := os.ReadFile(profileFilename)
	if err != nil {
		return nil, err
	}

	p, err := profile.Parse(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("parse profile: %w", err)
	}

	index := heatmap.NewIndex(config)
	if err := index.AddProfile(p); err != nil {
		return nil, fmt.Errorf("add profile to index: %w", err)
	}

	return index, nil
}
