import js from '@eslint/js';
import svelte from 'eslint-plugin-svelte';
import globals from 'globals';
import ts from 'typescript-eslint';
import prettier from 'eslint-config-prettier';
import svelteConfig from './svelte.config.js';

// Phase 141 — file-length ratchet via the linter's own `max-lines` rule (the
// idiomatic home; no custom hook). Threshold = 530 non-blank LOC (500 + a
// ±20-30 tolerance — operator decision 2026-06-15: never fragment cohesion to
// force <500). Counts the WHOLE file, so for a .svelte component it is
// script + markup + scoped CSS. Production TS + Svelte only; tests are out of
// scope (the long test files are tracked for Phase 142). NEW files over the
// threshold fail; existing residuals are capped at their current size below so
// they can never GROW.
const FILE_LENGTH_MAX = 530;

// Justified residuals (Phase 141). Each file's pure/testable logic has already
// been extracted to a companion module (+unit tests) or shared, OR the file is
// irreducible imperative render glue, a data table, or a cohesive entrypoint —
// the remaining length is markup + scoped CSS + interaction glue a length split
// would only fragment. `max` = current non-blank LOC (a no-growth cap). Burn
// down by lowering `max` as Tier-2b markup sub-components are extracted (now
// safe behind the Playwright e2e net); delete the entry once the file is ≤530.
// Filename globs are unique across the repo; the route page uses a path glob
// because `+page.svelte` is not unique and `(app)`/`[id]` are glob-special.
const FILE_LENGTH_ALLOWLIST = [
  // PanelControls.svelte + PanelHost.svelte + L5EvidenceReader.svelte —
  // decomposed in Phase 141 into per-lever (./levers/*) and per-region children
  // (PanelToolbar / PanelScopeChips / PanelDisclosureNotes / PanelCellGrid /
  // PanelCell; L5MetaGrid / L5NegativeSpaceSection / L5DiffTab /
  // L5RevisionHistory + pure logic in l5-evidence-internals.ts, unit-tested)
  // each <530; all parents are now thin orchestrators under the global cap
  // (no entry needed).
  ['**/CoOccurrenceNetworkCell.svelte', 1222], // logic in cooccurrence-network-shared.ts (tested); residual = d3-force/SVG + pan/zoom glue
  ['**/ScopeEditor.svelte', 997], // scoped-CSS 456 dominated; draft logic in scope-editor-draft.ts; ScopeGroupCard split = Tier-2b
  ['**/AnalysesOverlay.svelte', 974], // async-API orchestration + markup + scoped-CSS; AnalysisRow/ShareDrawer split = Tier-2b
  ['**/packages/engine-3d/src/engine.ts', 937], // imperative Three.js/WebGL engine; E2E-covered; not logic-decomposable
  ['**/open-questions.ts', 743], // DATA table (open research-question content), not logic; relocate-to-JSON deferred
  ['**/CoOccurrenceNetworkAtScale.svelte', 728], // logic in cooccurrence-network-shared.ts (tested); residual = sigma/FA2/WebGL glue
  ['**/CellConfigPopover.svelte', 660], // per-configurableParams field-renderer markup; field-renderer split = Tier-2b
  ['**/reflection/wp/*/+page.svelte', 642], // markdown/section rendering markup; section-renderer split = Tier-2b
  ['**/AtmosphereSurface.svelte', 642], // transforms → atmosphere-surface-internals.ts (+8t); residual = markup + scoped-CSS 239 + handlers
  ['**/ProbeCard.svelte', 568], // capability-matrix markup; matrix child split = Tier-2b
  ['**/SideRail.svelte', 533] // markup-dominated; 3 LOC over — within the ±20-30 tolerance band
];

export default ts.config(
  js.configs.recommended,
  ...ts.configs.recommended,
  ...svelte.configs.recommended,
  prettier,
  ...svelte.configs.prettier,
  {
    languageOptions: {
      globals: {
        ...globals.browser,
        ...globals.node
      }
    },
    rules: {
      '@typescript-eslint/no-unused-vars': [
        'error',
        { argsIgnorePattern: '^_', varsIgnorePattern: '^_' }
      ],
      // Retired-vocabulary ratchet (Phase 140). Bans the old domain terms from
      // re-entering the code as identifiers or user-facing strings. Comments are
      // not AST nodes, so legitimate historical "retired X" notes are untouched.
      // The lowercase URL key 'viewMode' is a contract (legacy /lanes/ redirect)
      // and stays — it is a string Literal but does not match these patterns.
      // See docs/development/conventions.md §1.
      'no-restricted-syntax': [
        'error',
        {
          selector: 'Identifier[name=/^(ViewMode|ViewingMode|WorkbenchScopeBar)/]',
          message:
            'Retired vocabulary (Phase 140): use `Presentation` (not ViewMode) and `PillarId` (not ViewingMode); the WorkbenchScopeBar component was removed. See docs/development/conventions.md §1.'
        },
        {
          selector: 'Literal[value=/Function Lane/]',
          message:
            'Retired surface name (Phase 140): the second surface is the Workbench. See docs/development/conventions.md §1.'
        },
        {
          selector: 'TemplateElement[value.raw=/Function Lane/]',
          message:
            'Retired surface name (Phase 140): the second surface is the Workbench. See docs/development/conventions.md §1.'
        }
      ]
    }
  },
  {
    // This config necessarily contains the retired terms inside the
    // no-restricted-syntax selector patterns/messages; exempt it from its own
    // rule so it does not flag itself.
    files: ['eslint.config.js'],
    rules: { 'no-restricted-syntax': 'off' }
  },
  {
    files: ['**/*.svelte', '**/*.svelte.ts', '**/*.svelte.js'],
    languageOptions: {
      parserOptions: {
        projectService: true,
        extraFileExtensions: ['.svelte'],
        parser: ts.parser,
        svelteConfig
      }
    }
  },
  // Phase 141 — file-length ratchet (production TS + Svelte). `skipBlankLines`
  // makes this count non-blank LOC; comments ARE counted (they carry real
  // maintenance weight). NEW files over FILE_LENGTH_MAX fail here.
  {
    files: ['**/*.ts', '**/*.svelte', '**/*.svelte.ts'],
    rules: {
      'max-lines': ['error', { max: FILE_LENGTH_MAX, skipBlankLines: true, skipComments: false }]
    }
  },
  // Per-file caps for the justified residuals (raise the limit to their current
  // size — a no-growth ceiling). Ordered after the global rule so they win.
  ...FILE_LENGTH_ALLOWLIST.map(([file, max]) => ({
    files: [file],
    rules: {
      'max-lines': ['error', { max, skipBlankLines: true, skipComments: false }]
    }
  })),
  // Tests are out of the Phase-141 file-length scope (long test files → Phase
  // 142); the ratchet covers production source only.
  {
    files: ['**/*.test.ts', '**/*.spec.ts', 'tests/**'],
    rules: { 'max-lines': 'off' }
  },
  {
    ignores: [
      'build/',
      '.svelte-kit/',
      'dist/',
      'node_modules/',
      'playwright-report/',
      'test-results/',
      'coverage/',
      // Generated by `make codegen-ts`.
      'src/lib/api/types.ts'
    ]
  }
);
