package playgroundsamples

import "github.com/odvcencio/gotreesitter-docs/internal/playgroundengine"

// Starter samples are shared by the server-rendered playground shell and the
// browser engine. Each curated sample is parsed in samples_test.go so a broken
// source or query fails before it reaches a visitor.
//
// Each Query is deliberately simple and uses node names that exist in that
// grammar. Adding a language here without a working query is worse than
// omitting it, because the playground opens on a query error.

type Sample struct {
	Source string
	Query  string
}

var languageSamples = map[string]Sample{
	"go": {
		Source: `package main

import "fmt"

func greet(name string) string {
	return "hey, " + name
}

func main() {
	fmt.Println(greet("gopher"))
}
`,
		Query: `(function_declaration name: (identifier) @function)`,
	},
	"python": {
		Source: `def greet(name):
    return f"hey, {name}"


class Greeter:
    def __init__(self, name):
        self.name = name

    def speak(self):
        return greet(self.name)
`,
		Query: `(function_definition name: (identifier) @function)`,
	},
	"javascript": {
		Source: `function greet(name) {
  return ` + "`hey, ${name}`" + `;
}

const shout = (name) => greet(name).toUpperCase();

console.log(shout("world"));
`,
		Query: `(function_declaration name: (identifier) @function)`,
	},
	"typescript": {
		Source: `interface Person {
  name: string;
  age?: number;
}

function greet(person: Person): string {
  return ` + "`hey, ${person.name}`" + `;
}

const shout = (p: Person): string => greet(p).toUpperCase();
`,
		Query: `(function_declaration name: (identifier) @function)`,
	},
	"rust": {
		Source: `struct Greeter {
    name: String,
}

impl Greeter {
    fn new(name: &str) -> Self {
        Greeter { name: name.to_string() }
    }

    fn speak(&self) -> String {
        format!("hey, {}", self.name)
    }
}

fn main() {
    println!("{}", Greeter::new("rustacean").speak());
}
`,
		Query: `(function_item name: (identifier) @function)`,
	},
	"java": {
		Source: `public class Greeter {
    private final String name;

    public Greeter(String name) {
        this.name = name;
    }

    public String speak() {
        return "hey, " + name;
    }
}
`,
		Query: `(method_declaration name: (identifier) @method)`,
	},
	"c": {
		Source: `#include <stdio.h>

static const char *greet(const char *name) {
    static char buffer[64];
    snprintf(buffer, sizeof buffer, "hey, %s", name);
    return buffer;
}

int main(void) {
    puts(greet("world"));
    return 0;
}
`,
		Query: `(function_definition declarator: (function_declarator declarator: (identifier) @function))`,
	},
	"cpp": {
		Source: `#include <string>
#include <iostream>

class Greeter {
public:
    explicit Greeter(std::string name) : name_(std::move(name)) {}

    std::string speak() const { return "hey, " + name_; }

private:
    std::string name_;
};

int main() {
    std::cout << Greeter("world").speak() << '\n';
}
`,
		Query: `(class_specifier name: (type_identifier) @class)`,
	},
	"ruby": {
		Source: `class Greeter
  def initialize(name)
    @name = name
  end

  def speak
    "hey, #{@name}"
  end
end

puts Greeter.new("world").speak
`,
		Query: `(method name: (identifier) @method)`,
	},
	"json": {
		Source: `{
  "name": "gotreesitter",
  "parsers": 206,
  "pure_go": true,
  "targets": ["wasm", "native"],
  "limits": { "max_source_bytes": 65536 }
}
`,
		Query: `(pair key: (string) @key)`,
	},
	"bash": {
		Source: `#!/usr/bin/env bash
set -euo pipefail

greet() {
  local name="$1"
  printf 'hey, %s\n' "$name"
}

for who in world gopher; do
  greet "$who"
done
`,
		Query: `(function_definition name: (word) @function)`,
	},
	"css": {
		Source: `:root {
  --paper: #f7f4ec;
  --ink: #14151b;
}

.playground .panel {
  background: var(--paper);
  border-radius: 2px;
}

@media (max-width: 720px) {
  .playground .panel { padding: 8px; }
}
`,
		Query: `(rule_set (selectors) @selector)`,
	},
}

// All returns a copy of the curated starter samples.
func All() map[string]Sample {
	out := make(map[string]Sample, len(languageSamples))
	for language, sample := range languageSamples {
		out[language] = sample
	}
	return out
}

// ForLanguage returns a curated starter sample, if one exists.
func ForLanguage(language string) (Sample, bool) {
	sample, ok := languageSamples[language]
	return sample, ok
}

// ForIndexLanguage returns a language-specific starter sample for any
// canonical grammar name in the playground index. Languages without a
// curated sample use the first comment form that parses cleanly.
func ForIndexLanguage(language string) (Sample, bool) {
	if sample, ok := ForLanguage(language); ok {
		return sample, true
	}
	for _, source := range genericSources {
		result := playgroundengine.Parse(source, "", language, false)
		if result.ParseError == "" && !result.HasErrors && result.NodeCount > 0 {
			return Sample{Source: source}, true
		}
	}
	return Sample{}, false
}

// GenericSources returns the comment candidates used when a grammar has no
// curated example.
func GenericSources() []string {
	return append([]string(nil), genericSources...)
}

var genericSources = []string{
	"// gotreesitter sample\n",
	"# gotreesitter sample\n",
	"-- gotreesitter sample\n",
	"/* gotreesitter sample */\n",
	"<!-- gotreesitter sample -->\n",
	"% gotreesitter sample\n",
	"; gotreesitter sample\n",
}
