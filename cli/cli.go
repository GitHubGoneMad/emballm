package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"emballm/internal/scans"
	"emballm/internal/scans/results"
	"emballm/internal/services"
	"emballm/internal/services/ollama"
	"emballm/internal/services/vertex"
)

func Command(release string) {
	fmt.Println(release)

	err := CheckRequirements()
	if err != nil {
		Log.Error("checking requirements: %v", err)
		return
	}

	flags, err := ParseFlags()
	if err != nil {
		Log.Error("parsing flags: %v", err)
		return
	}

	var gatherType, scanPath string
	if flags.Directory != "" {
		gatherType = scans.ScanTypes.Directory
		scanPath = flags.Directory
	} else if flags.File != "" {
		gatherType = scans.ScanTypes.File
		scanPath = flags.File
	}

	fmt.Println(fmt.Sprintf("Scanning %s: %s", gatherType, scanPath))

	fileScans, err := scans.GatherFiles(gatherType, scanPath, flags.Exclude)
	if err != nil {
		Log.Error("gathering files: %v", err)
		return
	}

	var result []results.Issue
	switch flags.Service {
	case services.Supported.Ollama:
		result, err = ollama.Scan(ollama.ScanClient{Model: flags.Model}, fileScans)
		if err != nil {
			Log.Error("scanning: %v", err)
			return
		}

	case services.Supported.Vertex:
		var scan string
		var waitGroup sync.WaitGroup

		for _, fileScan := range fileScans {
			waitGroup.Add(1)
			go func(fileScan *scans.FileScan) {
				defer waitGroup.Done()
				fileResult, err := vertex.Scan(flags.Model, fileScan.Path)
				if err != nil {
					Log.Error("scanning: %v", err)
					return
				}
				scan += *fileResult
				fileScan.Status = scans.Status.Complete
			}(fileScan)
		}

		waitGroup.Wait()
		result = []results.Issue{}
	default:
		Log.Error("unknown service: %s", flags.Service)
		return
	}
	// Marshal the struct into JSON
	jsonData, err := json.MarshalIndent(result, "", "    ")
	if err != nil {
		Log.Warn("marshaling JSON:", err)

	}
	err = os.WriteFile(flags.Output, jsonData, 0644)
	if err != nil {
		Log.Error("writing output: %v", err)
		return
	}
}
