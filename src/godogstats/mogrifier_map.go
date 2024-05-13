package godogstats

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/kelseyhightower/envconfig"
)

var varFinder = regexp.MustCompile(`\$\d+`) // matches $0, $1, etc.

const envPrefix = "DOG_STATSD_MOGRIFIER" // prefix for environment variables

// mogrifierMap is a map of regular expressions to functions that mogrify a name and return tags
type mogrifierMap map[*regexp.Regexp]func([]string) (string, []string)

// makePatternHandler returns a function that replaces $0, $1, etc. in the pattern with the corresponding match
func makePatternHandler(pattern string) func([]string) string {
	return func(matches []string) string {
		return varFinder.ReplaceAllStringFunc(pattern, func(s string) string {
			i, _ := strconv.Atoi(s[1:])
			return matches[i]
		})
	}
}

// newMogrifierMapFromEnv loads mogrifiers from environment variables
// keys is a list of mogrifier names to load
func newMogrifierMapFromEnv(keys []string) (mogrifierMap, error) {
	mogrifiers := mogrifierMap{}

	type config struct {
		Pattern string            `envconfig:"PATTERN"`
		Tags    map[string]string `envconfig:"TAGS"`
		Name    string            `envconfig:"NAME"`
	}

	for _, mogrifier := range keys {
		cfg := config{}
		if err := envconfig.Process(envPrefix+"_"+mogrifier, &cfg); err != nil {
			return nil, fmt.Errorf("failed to load mogrifier %s: %v", mogrifier, err)
		}

		re, err := regexp.Compile(cfg.Pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to compile pattern for %s: %s: %v", mogrifier, cfg.Pattern, err)
		}

		nameHandler := makePatternHandler(cfg.Name)
		tagHandlers := make(map[string]func([]string) string, len(cfg.Tags))
		for key, value := range cfg.Tags {
			tagHandlers[key] = makePatternHandler(value)
		}

		mogrifiers[re] = func(matches []string) (string, []string) {
			name := nameHandler(matches)
			tags := make([]string, 0, len(tagHandlers))
			for tagKey, handler := range tagHandlers {
				tagValue := handler(matches)
				tags = append(tags, tagKey+":"+tagValue)
			}
			return name, tags
		}

	}
	return mogrifiers, nil
}

// mogrify applies the first mogrifier in the map that matches the name
func (m mogrifierMap) mogrify(name string) (string, []string) {
	if m == nil {
		return name, nil
	}
	for matcher, mogrifier := range m {
		matches := matcher.FindStringSubmatch(name)
		if len(matches) == 0 {
			continue
		}

		mogrifiedName, tags := mogrifier(matches)
		return mogrifiedName, tags
	}

	// no mogrification
	return name, nil
}
