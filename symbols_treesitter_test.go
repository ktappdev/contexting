package contexting

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/odvcencio/gotreesitter/grammars"
)

func TestTreeSitterExtraction(t *testing.T) {
	dir := t.TempDir()

	files := map[string]string{
		"sample.py": `class UserService:
    def __init__(self, db):
        self.db = db

    async def fetch_user(self, user_id):
        return await self.db.get(user_id)

    def _validate_email(self, email):
        return "@" in email

def create_app(config):
    return App(config)
`,
		"sample.ts": `export class AuthHandler {
    private token: string;

    constructor(token: string) {
        this.token = token;
    }

    async refreshToken(): Promise<string> {
        return "ok";
    }

    private validateScope(scope: string): boolean {
        return true;
    }
}

export const JWT_EXPIRY = 3600;

export interface TokenPayload {
    sub: string;
}

export type AuthResult = { token: string };

async function loginUser(credentials): Promise<AuthResult> {
    return { token: "abc" };
}
`,
		"sample.rs": `pub struct HttpClient {
    client: reqwest::Client,
    base_url: String,
}

impl HttpClient {
    pub fn new(base_url: String) -> Self {
        Self { client: reqwest::Client::new(), base_url }
    }

    pub async fn get(&self, path: &str) -> Result<Response, Error> {
        Ok(Response::new())
    }
}

pub fn create_client(url: &str) -> HttpClient {
    HttpClient::new(url.to_string())
}
`,
		"sample.js": `export class AuthHandler {
    constructor(token) {
        this.token = token;
    }

    async refreshToken() {
        return "ok";
    }
}

export const loginUser = async (creds) => {
    return { token: "abc" };
};
`,
		"sample.svelte": `<script lang="ts">
    interface UserData {
        name: string;
        id: number;
    }

    async function fetchUser(id: number): Promise<UserData> {
        return { name: "test", id };
    }

    class UserStore {
        private users: UserData[] = [];

        add(user: UserData): void {
            this.users.push(user);
        }
    }
</script>

<h1>User List</h1>
`,
		"sample.astro": `---
import Layout from '../layouts/Layout.astro';

interface PageProps {
    title: string;
}

const { title } = Astro.props;

async function getInitialData() {
    return { users: [] };
}

const data = await getInitialData();
---

<Layout title={title}>
    <h1>Users</h1>
</Layout>
`,
	}

	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := writeFile(path, content); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	// Force tree-sitter mode for this test
	prev := SymbolsExtractorMode
	SymbolsExtractorMode = "treesitter"
	defer func() { SymbolsExtractorMode = prev }()

	tests := []struct {
		file     string
		mustHave []string
	}{
		{
			file: "sample.py",
			mustHave: []string{
				"UserService",      // class
				"__init__",         // method (was missed by regex)
				"fetch_user",       // method (was missed by regex)
				"_validate_email",  // method (was missed by regex)
				"create_app",       // top-level function
			},
		},
		{
			file: "sample.ts",
			mustHave: []string{
				"AuthHandler",       // class
				"refreshToken",      // method (was missed by regex)
				"validateScope",     // method (was missed by regex)
				"TokenPayload",      // interface
				"AuthResult",        // type alias
				"loginUser",         // function
			},
		},
		{
			file: "sample.rs",
			mustHave: []string{
				"HttpClient",     // struct
				"new",            // method in impl (was missed by regex)
				"get",            // method in impl (was missed by regex)
				"create_client",  // top-level function
			},
		},
		{
			file: "sample.js",
			mustHave: []string{
				"AuthHandler",  // class
				"refreshToken", // method
				"loginUser",    // arrow function const
			},
		},
		{
			file: "sample.svelte",
			mustHave: []string{
				"UserData",    // interface (from <script lang="ts">)
				"fetchUser",   // function
				"UserStore",   // class
				"add",         // method
			},
		},
		{
			file: "sample.astro",
			mustHave: []string{
				"PageProps",       // interface
				"getInitialData", // function in frontmatter
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.file, func(t *testing.T) {
			syms, err := extractSymbolsTreeSitter(filepath.Join(dir, tc.file))
			if err != nil {
				t.Fatalf("extractSymbolsTreeSitter(%s): %v", tc.file, err)
			}
			got := make(map[string]bool)
			for _, s := range syms {
				got[s] = true
			}
			sort.Strings(syms)
			t.Logf("symbols: %v", syms)
			for _, want := range tc.mustHave {
				if !got[want] {
					t.Errorf("missing expected symbol %q (got: %v)", want, syms)
				}
			}
		})
	}
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

func TestExtractFileImports(t *testing.T) {
	dir := t.TempDir()

	files := map[string]string{
		// The motivating case: a Next.js webhook route that imports Clerk.
		"webhook.ts": `import { Webhook } from "@clerk/nextjs";
import { NextResponse } from "next/server";
import type { WebhookEvent } from "@clerk/nextjs/server";
import { db } from "./db";

export async function POST(req: Request) {
  return new Response("ok");
}
`,
		// Duplicated imports should be deduplicated.
		"dupes.ts": `import x from "react";
import y from "react";
import z from "react";
`,
		// `import type` is also an import statement — should be captured.
		"types.ts": `import type { User } from "./types";
import type { Post } from "../models/post";
`,
		// JS file with CJS-style require — only ESM imports are captured.
		// This documents the current behavior: CommonJS `require()` is not
		// captured by the import_statement query.
		"commonjs.js": `const lodash = require("lodash");
import x from "react";
`,
		// Svelte with TypeScript.
		"svelte_ts.svelte": `<script lang="ts">
import { auth } from "@clerk/nextjs";
import { writable } from "svelte/store";
</script>

<h1>Hello</h1>
`,
		// Svelte with plain JS.
		"svelte_js.svelte": `<script>
import { onMount } from "svelte";
import axios from "axios";
</script>
`,
		// File with no imports.
		"empty.ts": `export const foo = 1;
export function bar() { return foo; }
`,
	}

	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := writeFile(path, content); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	tests := []struct {
		file     string
		mustHave []string
		mustMiss []string
	}{
		{
			file: "webhook.ts",
			mustHave: []string{
				"@clerk/nextjs",
				"next/server",
				"@clerk/nextjs/server",
				"./db",
			},
		},
		{
			file:     "dupes.ts",
			mustHave: []string{"react"},
			// Should be deduplicated to a single entry.
			mustMiss: []string{},
		},
		{
			file: "types.ts",
			mustHave: []string{
				"./types",
				"../models/post",
			},
		},
		{
			file: "commonjs.js",
			// ESM import captured, CJS require() is intentionally not.
			mustHave: []string{"react"},
			mustMiss: []string{"lodash"},
		},
		{
			file: "svelte_ts.svelte",
			mustHave: []string{
				"@clerk/nextjs",
				"svelte/store",
			},
		},
		{
			file: "svelte_js.svelte",
			mustHave: []string{
				"svelte",
				"axios",
			},
		},
		{
			file:     "empty.ts",
			mustHave: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.file, func(t *testing.T) {
			imps := extractFileImports(filepath.Join(dir, tc.file))
			t.Logf("imports: %v", imps)
			got := make(map[string]bool)
			for _, s := range imps {
				got[s] = true
			}
			for _, want := range tc.mustHave {
				if !got[want] {
					t.Errorf("missing expected import %q (got: %v)", want, imps)
				}
			}
			for _, miss := range tc.mustMiss {
				if got[miss] {
					t.Errorf("unexpected import %q present (got: %v)", miss, imps)
				}
			}
		})
	}

	t.Run("dedup", func(t *testing.T) {
		imps := extractFileImports(filepath.Join(dir, "dupes.ts"))
		if len(imps) != 1 {
			t.Errorf("expected 1 import (deduped), got %d: %v", len(imps), imps)
		}
	})

	t.Run("unsupported extension returns nil", func(t *testing.T) {
		path := filepath.Join(dir, "noop.py")
		if err := writeFile(path, "import x\n"); err != nil {
			t.Fatalf("write py: %v", err)
		}
		if imps := extractFileImports(path); imps != nil {
			t.Errorf("expected nil for .py, got %v", imps)
		}
	})

	t.Run("missing file returns nil", func(t *testing.T) {
		if imps := extractFileImports(filepath.Join(dir, "does-not-exist.ts")); imps != nil {
			t.Errorf("expected nil for missing file, got %v", imps)
		}
	})

	t.Run("no imports returns nil", func(t *testing.T) {
		imps := extractFileImports(filepath.Join(dir, "empty.ts"))
		if imps != nil {
			t.Errorf("expected nil for file with no imports, got %v", imps)
		}
	})
}

func TestExtractImportsFromSource(t *testing.T) {
	// Direct test of the parsing helper without going through the file system.
	lg := grammars.TypescriptLanguage()
	src := []byte(`import { auth } from "@clerk/nextjs";
import "side-effect";
import x from "react";
`)
	imps := extractImportsFromSource(src, lg)
	want := []string{"@clerk/nextjs", "side-effect", "react"}
	if len(imps) != len(want) {
		t.Fatalf("expected %d imports, got %d: %v", len(want), len(imps), imps)
	}
	for i, w := range want {
		if imps[i] != w {
			t.Errorf("import[%d]: expected %q, got %q", i, w, imps[i])
		}
	}
}
