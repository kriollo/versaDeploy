package provision

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// fallbackVersions are used offline. ponytail: update them on new releases; the live
// list from endoflife.date replaces them whenever it can be fetched.
var fallbackVersions = map[string][]string{
	"php":    {"8.5", "8.4", "8.3", "8.2"},
	"node":   {"26", "24", "22"}, // LTS only
	"python": {"3.15", "3.14", "3.13", "3.12", "3.11"},
}

var (
	versionsMu   sync.Mutex
	liveVersions = map[string][]string{}
)

// Versions returns the supported versions of php, node or python, newest first, at
// least three. It never blocks: it uses what FetchVersions got, or the fallback list.
func Versions(product string) []string {
	versionsMu.Lock()
	defer versionsMu.Unlock()
	v := liveVersions[product]
	for _, f := range fallbackVersions[product] {
		if len(v) >= 3 {
			break
		}
		if !containsStr(v, f) {
			v = append(v, f)
		}
	}
	return v
}

var fetchOnce sync.Once

// FetchVersions loads the supported release cycles from endoflife.date once per process
// (a few KB, 4s timeout). Failures keep the fallback lists.
func FetchVersions() {
	fetchOnce.Do(func() {
		c := &http.Client{Timeout: 4 * time.Second}
		for product, api := range map[string]string{"php": "php", "node": "nodejs", "python": "python"} {
			resp, err := c.Get("https://endoflife.date/api/" + api + ".json")
			if err != nil {
				continue
			}
			var cycles []eolCycle
			err = json.NewDecoder(resp.Body).Decode(&cycles)
			resp.Body.Close()
			if err != nil {
				continue
			}
			if v := supportedCycles(cycles, product == "node", time.Now()); len(v) > 0 {
				versionsMu.Lock()
				liveVersions[product] = v
				versionsMu.Unlock()
			}
		}
	})
}

// eolCycle is one endoflife.date release cycle; eol and lts are a date or a bool.
type eolCycle struct {
	Cycle       string `json:"cycle"`
	ReleaseDate string `json:"releaseDate"`
	EOL         any    `json:"eol"`
	LTS         any    `json:"lts"`
}

// supportedCycles keeps the released, not end-of-life cycles (only LTS ones when
// ltsOnly), newest first as the API lists them.
func supportedCycles(cycles []eolCycle, ltsOnly bool, now time.Time) []string {
	today := now.Format("2006-01-02")
	var out []string
	for _, c := range cycles {
		if c.ReleaseDate > today {
			continue
		}
		if eol, ok := c.EOL.(string); ok && eol <= today {
			continue
		}
		if eol, ok := c.EOL.(bool); ok && eol {
			continue
		}
		if ltsOnly {
			lts, isDate := c.LTS.(string)
			if !isDate || lts > today {
				if b, ok := c.LTS.(bool); !ok || !b {
					continue
				}
			}
		}
		out = append(out, c.Cycle)
	}
	return out
}

func containsStr(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
