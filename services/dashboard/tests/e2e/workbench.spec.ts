import { expect, test, type Route, type Page } from './_fixtures';

// Phase 141 — Workbench PanelControls characterization net.
//
// The safety harness for the PanelControls decomposition (constraint #3 of the
// giants refactor): it pins the per-lever control strip's RENDERED structure
// and the click→URL reactivity that a markup/CSS sub-componentisation could
// silently break. It is deliberately behavioural, not pixel-based — it must stay
// green across every child-component extraction while the internal markup moves.
//
// Seeds the canonical three-surface grammar directly:
//   /workbench?activePillar=aleph&aleph=<base64url-json>
// where the decoded PillarState is a single Aleph Window with one focused
// distribution Panel over Probe 0:
//   { w:[{ p:[{ s:[{ pi:['probe-0-de-institutional-web'], si:[] }],
//                  c:'m', v:'distribution', m:'sentiment_score_sentiws',
//                  l:'g' }], fi:0 }], aw:0 }
// (computed once via encodePillarState; hardcoded so the spec needs no app-code
// import). Phase 149 (Zen 3.1) — PanelControls renders on EVERY panel, collapsed
// by default, so the strip region is present on load but its levers need an
// explicit expand (see `expandControls`).
//
// All BFF routes are mocked so the test runs against `pnpm preview` with no live
// backend (mirrors atmosphere.spec.ts / topic-views.spec.ts).

const PROBE_ID = 'probe-0-de-institutional-web';

// encodePillarState({ windows:[{ panels:[{ scopes:[{ probeIds:[PROBE_ID],
//   sourceIds:[] }], composition:'merged', view:'distribution',
//   metric:'sentiment_score_sentiws', layer:'gold' }], focusedPanelIndex:0 }],
//   activeWindowIndex:0 })
const ALEPH_SEED =
  'eyJ3IjpbeyJwIjpbeyJzIjpbeyJwaSI6WyJwcm9iZS0wLWRlLWluc3RpdHV0aW9uYWwtd2ViIl0sInNpIjpbXX1dLCJjIjoibSIsInYiOiJkaXN0cmlidXRpb24iLCJtIjoic2VudGltZW50X3Njb3JlX3NlbnRpd3MiLCJsIjoiZyJ9XSwiZmkiOjB9XSwiYXciOjB9';

const WORKBENCH_URL = `/workbench?activePillar=aleph&aleph=${ALEPH_SEED}`;

const probesPayload = [
  {
    probeId: PROBE_ID,
    displayName: 'Probe 0 — German institutional web',
    shortName: 'Probe 0',
    language: 'de',
    sources: ['tagesschau', 'bundesregierung'],
    emissionPoints: [{ latitude: 52.52, longitude: 13.405, label: 'Berlin' }]
  }
];

const dossierPayload = {
  probeId: PROBE_ID,
  language: 'de',
  windowStart: '2026-04-20T00:00:00Z',
  windowEnd: '2026-04-27T00:00:00Z',
  functionCoverage: {
    covered: 2,
    total: 4,
    functions: ['epistemic_authority', 'power_legitimation']
  },
  sources: [
    {
      name: 'tagesschau',
      type: 'web',
      url: 'https://www.tagesschau.de',
      articlesTotal: 100,
      articlesInWindow: 20,
      publicationFrequencyPerDay: 5,
      primaryFunction: 'epistemic_authority',
      secondaryFunction: null,
      emicDesignation: 'Public broadcaster',
      emicContext: 'German public media',
      silverEligible: true
    },
    {
      name: 'bundesregierung',
      type: 'web',
      url: 'https://www.bundesregierung.de',
      articlesTotal: 40,
      articlesInWindow: 8,
      publicationFrequencyPerDay: 1,
      primaryFunction: 'power_legitimation',
      secondaryFunction: null,
      emicDesignation: 'Federal government',
      emicContext: 'German executive press office',
      silverEligible: true
    }
  ]
};

const availableMetricsPayload = [
  { metricName: 'sentiment_score_sentiws', validationStatus: 'unvalidated' },
  { metricName: 'word_count', validationStatus: 'unvalidated' }
];

const scopeAvailableMetricsPayload = {
  scopedSources: ['tagesschau', 'bundesregierung'],
  available: ['sentiment_score_sentiws', 'word_count'],
  partial: []
};

const scopeAvailableMetadataPayload = {
  scopedSources: ['tagesschau', 'bundesregierung'],
  available: [],
  partial: []
};

const distributionPayload = {
  metricName: 'sentiment_score_sentiws',
  scope: 'probe',
  scopeId: PROBE_ID,
  windowStart: '2026-04-20T00:00:00Z',
  windowEnd: '2026-04-27T00:00:00Z',
  clampedUpper: 1,
  overflowCount: 0,
  bins: [
    { lower: -1, upper: 0, count: 12 },
    { lower: 0, upper: 1, count: 30 }
  ],
  summary: {
    count: 42,
    min: -1,
    max: 1,
    mean: 0.1,
    median: 0.05,
    p05: -0.8,
    p25: -0.2,
    p75: 0.4,
    p95: 0.9
  }
};

function genericContent(entityId: string) {
  return {
    entityId,
    entityType: 'view_mode',
    locale: 'en',
    contentVersion: 'v2026-05-test',
    lastReviewedBy: 'e2e-fixture',
    lastReviewedAt: '2026-05-01',
    registers: {
      semantic: { short: 'Mock semantic.', long: 'Mock semantic long.' },
      methodological: { short: 'Mock methodological.', long: 'Mock methodological long.' }
    }
  };
}

const json = (body: unknown) => ({
  status: 200,
  contentType: 'application/json',
  body: JSON.stringify(body)
});

async function mockBff(page: Page) {
  // Catch-all FIRST: an UNMOCKED endpoint 401s against the backend-less preview,
  // and any data-layer 401 trips the global unauthenticated redirect
  // (/workbench → /login → /), so the Workbench would never mount. A blanket 200
  // makes that impossible; the specific shaped routes below win (last-wins). The
  // auth/me re-assert restores the fixture's active session over the catch-all.
  await page.route('**/api/v1/**', (route: Route) => route.fulfill(json({})));
  await page.route('**/api/v1/auth/me', (route: Route) =>
    route.fulfill(
      json({ id: 'e2e-user', email: 'e2e@aer.test', role: 'researcher', status: 'active' })
    )
  );
  await page.route('**/api/v1/probes', (route: Route) => route.fulfill(json(probesPayload)));
  await page.route(`**/api/v1/probes/${PROBE_ID}/dossier**`, (route: Route) =>
    route.fulfill(json(dossierPayload))
  );
  await page.route('**/api/v1/content/**', (route: Route) => {
    const m = route
      .request()
      .url()
      .match(/content\/[^/]+\/([^?]+)/);
    const id = m?.[1] ? decodeURIComponent(m[1]) : 'unknown';
    return route.fulfill(json(genericContent(id)));
  });
  await page.route('**/api/v1/metrics/available**', (route: Route) =>
    route.fulfill(json(availableMetricsPayload))
  );
  await page.route('**/api/v1/scope/available-metrics**', (route: Route) =>
    route.fulfill(json(scopeAvailableMetricsPayload))
  );
  await page.route('**/api/v1/scope/available-metadata**', (route: Route) =>
    route.fulfill(json(scopeAvailableMetadataPayload))
  );
  // The distribution cell's data endpoint — kept 200 so the cell renders instead
  // of retry-storming a network error (irrelevant to the control-strip assertions).
  await page.route(/\/api\/v1\/metrics\/[^/]+\/distribution/, (route: Route) =>
    route.fulfill(json(distributionPayload))
  );
  // Metric provenance (CellMethodology) — valid shape so its `knownLimitations`
  // array read does not throw against the catch-all's `{}`.
  await page.route(/\/api\/v1\/metrics\/[^/]+\/provenance/, (route: Route) =>
    route.fulfill(
      json({
        metricName: 'sentiment_score_sentiws',
        tierClassification: 1,
        algorithmDescription: 'SentiWS lexical polarity (mock).',
        knownLimitations: [],
        validationStatus: 'unvalidated',
        extractorVersionHash: 'sha256:e2e-mock'
      })
    )
  );
  // Globe activity series (Phase 135 persistent globe) — empty.
  await page.route('**/api/v1/metrics?**', (route: Route) =>
    route.fulfill(json({ data: [], excludedCount: 0 }))
  );
}

// Phase 149 (Zen 3.1) — PanelControls now renders on every panel COLLAPSED by
// default (focus is a pure highlight, no layout jump). The lever assertions below
// need the strip open, so expand it first. Language-independent: clicks the
// control-strip header (its `aria-expanded` flips) rather than a localized label.
async function expandControls(page: Page) {
  const header = page.locator('.cell-controls-header').first();
  await expect(header).toBeVisible();
  if ((await header.getAttribute('aria-expanded')) === 'false') {
    await header.click();
    await expect(header).toHaveAttribute('aria-expanded', 'true');
  }
}

test.describe('Phase 141 — Workbench PanelControls characterization', () => {
  test.beforeEach(async ({ page }) => {
    await mockBff(page);
  });

  test('renders the per-lever control strip for a focused distribution panel', async ({ page }) => {
    await page.goto(WORKBENCH_URL);

    // The panel control strip is present on every panel (Phase 149); expand it so
    // the per-lever rows render.
    const strip = page.getByRole('region', { name: 'Panel controls' });
    await expect(strip).toBeVisible();
    await expandControls(page);

    // View lever — Phase 151: a dropdown (combobox) with Distribution selected.
    const viewSelect = strip.getByRole('combobox', { name: 'View' });
    await expect(viewSelect).toBeVisible();
    await expect(viewSelect).toHaveValue('distribution');

    // Composition lever — Merged active, Split offered.
    const compGroup = strip.getByRole('radiogroup', { name: 'Composition' });
    await expect(compGroup.getByRole('radio', { name: 'Merged', exact: true })).toHaveAttribute(
      'aria-checked',
      'true'
    );
    await expect(compGroup.getByRole('radio', { name: 'Split', exact: true })).toBeVisible();

    // Metric lever — Phase 151: a dropdown (combobox); sentiment is the bound metric.
    await expect(strip.getByRole('combobox', { name: 'Metric' })).toHaveValue(
      'sentiment_score_sentiws'
    );

    // Config lever — distribution declares `bins`, so the Histogram bins group renders.
    await expect(strip.getByRole('group', { name: 'Histogram bins' })).toBeVisible();
  });

  // Phase 144b — the View lever + panel eyebrow are driven by the conceptual-
  // vocabulary SoT (`presentations/registry.ts`), localized in this phase. With
  // `?lang=de` the registry accessors resolve German with no consumer change:
  // the View dropdown aria-label ("Darstellung"), its selected option and the
  // panel eyebrow all read "Verteilung" (Phase 151 — View is now a combobox).
  test('?lang=de localizes the registry-driven View lever + panel eyebrow', async ({ page }) => {
    await page.goto(`${WORKBENCH_URL}&lang=de`);
    await expandControls(page);

    const viewSelect = page.getByRole('combobox', { name: 'Darstellung' });
    await expect(viewSelect).toHaveValue('distribution');
    await expect(
      viewSelect.getByRole('option', { name: 'Verteilung', exact: true })
    ).toBeAttached();
    await expect(page.locator('article.panel-host .panel-eyebrow')).toHaveText('Verteilung');
  });

  test('clicking Split re-encodes the ?aleph= pillar state and reveals Direction', async ({
    page
  }) => {
    await page.goto(WORKBENCH_URL);
    await expandControls(page);

    const strip = page.getByRole('region', { name: 'Panel controls' });
    const compGroup = strip.getByRole('radiogroup', { name: 'Composition' });
    await expect(compGroup.getByRole('radio', { name: 'Merged', exact: true })).toHaveAttribute(
      'aria-checked',
      'true'
    );

    const alephBefore = new URL(page.url()).searchParams.get('aleph');
    expect(alephBefore).toBe(ALEPH_SEED);

    await compGroup.getByRole('radio', { name: 'Split', exact: true }).click();

    // The mutation re-encodes the Aleph pillar state into the URL (composition m→s).
    await expect.poll(() => new URL(page.url()).searchParams.get('aleph')).not.toBe(ALEPH_SEED);

    // Split is now active and the split-direction sub-lever appears.
    await expect(compGroup.getByRole('radio', { name: 'Split', exact: true })).toHaveAttribute(
      'aria-checked',
      'true'
    );
    await expect(strip.getByRole('radiogroup', { name: 'Split direction' })).toBeVisible();
  });

  // PanelHost decomposition net (Phase 141). The two tests above pin the
  // PanelControls strip; these pin PanelHost's OWN markup — the toolbar header
  // and the cell-grid fan-out — so a markup/CSS sub-componentisation of
  // PanelHost (PanelToolbar / PanelScopeChips / PanelCellGrid / PanelCell) stays
  // behaviour-preserving.
  test('renders the panel toolbar header + a single merged cell', async ({ page }) => {
    await page.goto(WORKBENCH_URL);

    const panel = page.locator('article.panel-host');
    await expect(panel).toBeVisible();

    // Toolbar: presentation eyebrow + bound metric + the scope-edit action.
    await expect(panel.locator('.panel-eyebrow')).toHaveText('Distribution');
    await expect(panel.locator('.panel-metric')).toHaveText('sentiment_score_sentiws');
    await expect(panel.getByRole('button', { name: 'Edit scope' })).toBeVisible();

    // Cell grid: a merged panel resolves to exactly one rendered cell, not split.
    await expect(panel.locator('.panel-body')).not.toHaveClass(/split/);
    await expect(panel.locator('.panel-cell')).toHaveCount(1);
  });

  test('Split fans the cell grid out to one cell per source', async ({ page }) => {
    await page.goto(WORKBENCH_URL);
    await expandControls(page);

    const panel = page.locator('article.panel-host');
    const strip = page.getByRole('region', { name: 'Panel controls' });
    const compGroup = strip.getByRole('radiogroup', { name: 'Composition' });

    // Merged → one cell.
    await expect(panel.locator('.panel-cell')).toHaveCount(1);

    await compGroup.getByRole('radio', { name: 'Split', exact: true }).click();

    // Split → the body takes the split layout and fans out per dossier source
    // (tagesschau + bundesregierung = 2 cells).
    await expect(panel.locator('.panel-body.split')).toBeVisible();
    await expect(panel.locator('.panel-cell')).toHaveCount(2);
  });
});

// ScopeEditor decomposition net (Phase 141). The PanelControls/PanelHost tests
// above never open the ScopeEditor — the Workbench's single configuration
// surface. These pin ITS rendered structure (the modal dialog, the first-class
// ScopeGroup cards, and the numbered probe/DF/source steps) and the two entry
// paths (⚙ Edit scope → edit-mode, ＋ Panel → create-mode) so a markup/CSS
// sub-componentisation of ScopeEditor (esp. a per-card ScopeGroupCard child)
// stays behaviour-preserving.
test.describe('Phase 141 — Workbench ScopeEditor characterization', () => {
  // The modal is `width: min(84rem, 100%)`, centred. At the default 1280-px
  // viewport it spans nearly full width, so its bottom-left footer button
  // (Add scope group) falls under the fixed SideRail (z-index 450 > modal 50).
  // A wider viewport centres the 84-rem modal clear of the ~184-px rail so the
  // genuine click lands — orthogonal to what this net pins.
  test.use({ viewport: { width: 1920, height: 1000 } });

  test.beforeEach(async ({ page }) => {
    await mockBff(page);
  });

  test('⚙ Edit scope opens the editor in edit-mode with the seed scope group card', async ({
    page
  }) => {
    await page.goto(WORKBENCH_URL);

    const panel = page.locator('article.panel-host');
    await panel.getByRole('button', { name: 'Edit scope' }).click();

    // The modal dialog mounts (shared aria-label across both modes).
    const dialog = page.getByRole('dialog', { name: 'Configure panel scope' });
    await expect(dialog).toBeVisible();
    // Edit-mode heading + Apply label distinguish it from create-mode.
    await expect(dialog.getByRole('heading', { name: 'Configure panel scope' })).toBeVisible();
    await expect(dialog.getByRole('button', { name: 'Apply changes' })).toBeVisible();

    // ScopeGroups are first-class visible cards — exactly one for the seed panel.
    const cards = dialog.locator('article.group');
    await expect(cards).toHaveCount(1);
    await expect(cards.first()).toHaveAttribute('aria-label', 'Scope group 1');

    // Step 1 renders Probe 0 as a chip, checked (the seed scope selects it).
    const probeChip = cards
      .first()
      .locator('.probe-chip', { hasText: 'Probe 0 — German institutional web' });
    await expect(probeChip).toBeVisible();
    await expect(probeChip).toHaveClass(/checked/);

    // Step 3 lists the probe's sources from the dossier (tagesschau + bundesregierung).
    await expect(dialog.locator('.source-row')).toHaveCount(2);

    // Cancel closes the editor (backdrop-click never does — only Esc / Cancel / Apply).
    await dialog.getByRole('button', { name: 'Cancel', exact: true }).click();
    await expect(dialog).toBeHidden();
  });

  test('＋ Panel opens create-mode; Add scope group appends a second card', async ({ page }) => {
    await page.goto(WORKBENCH_URL);

    // The WindowHost `＋ Panel` primary action opens the create-mode editor.
    // (Phase 127 added a second primary button — Save analysis — to the strip,
    // so target +Panel by its specific class.)
    await page.locator('button.panel-action-trailing').click();

    const dialog = page.getByRole('dialog', { name: 'Configure panel scope' });
    await expect(dialog).toBeVisible();
    // Create-mode heading + Apply label.
    await expect(dialog.getByRole('heading', { name: 'Configure new panel' })).toBeVisible();
    const applyBtn = dialog.getByRole('button', { name: 'Create panel' });
    // Create mode starts with one empty group → no probe yet → Apply disabled.
    await expect(dialog.locator('article.group')).toHaveCount(1);
    await expect(applyBtn).toBeDisabled();

    // Add a parallel scope group → a second first-class card appears.
    await dialog.getByRole('button', { name: 'Add scope group' }).click();
    const cards = dialog.locator('article.group');
    await expect(cards).toHaveCount(2);
    await expect(cards.nth(1)).toHaveAttribute('aria-label', 'Scope group 2');

    // Each card carries the three numbered steps (probes / DF / sources).
    await expect(cards.nth(1).locator('.step[data-step="1"]')).toBeVisible();
    await expect(cards.nth(1).locator('.step[data-step="2"]')).toBeVisible();
    await expect(cards.nth(1).locator('.step[data-step="3"]')).toBeVisible();
  });
});

// CellConfigPopover decomposition net (Phase 141). None of the tests above open
// the per-cell config popover — offered only on a multi-cell panel (Split). These
// pin ITS rendered structure (the dialog shell, the per-lever rows with their
// override dots, and the override → custom-note → Reset lifecycle) so a markup/CSS
// sub-componentisation of CellConfigPopover (per-type lever children) stays
// behaviour-preserving. The distribution seed declares `bins` + `scales` and (it
// usesMetric) a dimension peek, so a single cheap seed exercises all THREE popover
// control primitives — select (dimension) / slider (bins) / switch (scale) — plus
// the override path; the cooccurrence/scatter lever branches are structurally
// identical markup, so no extra d3 seed is needed.
test.describe('Phase 141 — Workbench CellConfigPopover characterization', () => {
  test.beforeEach(async ({ page }) => {
    await mockBff(page);
  });

  async function openFirstCellPopover(page: Page) {
    await page.goto(WORKBENCH_URL);
    await expandControls(page);
    const panel = page.locator('article.panel-host');
    const strip = page.getByRole('region', { name: 'Panel controls' });
    // Split → two source cells, each carrying the per-cell config affordance.
    await strip
      .getByRole('radiogroup', { name: 'Composition' })
      .getByRole('radio', { name: 'Split', exact: true })
      .click();
    await expect(panel.locator('.panel-cell')).toHaveCount(2);
    // Open the first cell's popover.
    await panel
      .locator('.panel-cell')
      .first()
      .getByRole('button', { name: 'Configure this cell' })
      .click();
    const dialog = page.getByRole('dialog', { name: /Cell configuration for/ });
    await expect(dialog).toBeVisible();
    return dialog;
  }

  test('⚙ Cell opens the per-cell popover with the distribution levers', async ({ page }) => {
    const dialog = await openFirstCellPopover(page);

    // Header eyebrow.
    await expect(dialog.getByText('Cell config')).toBeVisible();

    // Starts inheriting → muted note + Reset disabled.
    await expect(dialog.getByText('Inheriting the panel default for every lever.')).toBeVisible();
    await expect(dialog.getByRole('button', { name: /Reset to panel default/ })).toBeDisabled();

    // All three popover control primitives render for a distribution cell:
    //   dimension peek (select), bins (slider), scale (switch).
    await expect(dialog.getByRole('group', { name: 'Cell dimension' })).toBeVisible();
    await expect(dialog.getByRole('group', { name: 'Histogram bins' })).toBeVisible();
    await expect(dialog.getByRole('group', { name: 'Axis scale' })).toBeVisible();
  });

  test('Phase 128 a11y — popover takes focus on open, Esc closes it, focus returns', async ({
    page
  }) => {
    const trigger = page
      .locator('article.panel-host .panel-cell')
      .first()
      .getByRole('button', { name: 'Configure this cell' });
    const dialog = await openFirstCellPopover(page);
    // Focus moved INTO the dialog so the Escape handler (and Tab) reach it.
    await expect(dialog).toBeFocused();
    // Escape closes the popover and focus returns to the trigger it came from.
    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();
    await expect(trigger).toBeFocused();
  });

  test('toggling a lever overrides the cell, shows the dot, and Reset clears it', async ({
    page
  }) => {
    const dialog = await openFirstCellPopover(page);

    // Flip the Scale switch → the cell goes onto a custom config.
    const scaleGroup = dialog.getByRole('group', { name: 'Axis scale' });
    await expect(scaleGroup.locator('.ccp-dot')).toHaveCount(0);
    await scaleGroup.getByRole('switch').click();

    // Custom note replaces the inherit note; the Scale row shows its override dot.
    await expect(dialog.getByText(/not directly comparable to its sibling/)).toBeVisible();
    await expect(scaleGroup.locator('.ccp-dot')).toHaveCount(1);

    // Reset becomes enabled, clears the override, and closes the popover.
    const reset = dialog.getByRole('button', { name: /Reset to panel default/ });
    await expect(reset).toBeEnabled();
    await reset.click();
    await expect(dialog).toBeHidden();
  });
});
