-- Migration 032: Capitalise franceinfo's emic designation (Phase 148g).
--
-- Migration 021 seeded franceinfo's emic_designation as the brand's official
-- lower-case styling ('franceinfo'). Every sibling source reads as a proper noun
-- (Tagesschau, Bundesregierung, Élysée (Présidence de la République)), so the
-- lower-case form rendered inconsistently across the Workbench source-label
-- surfaces (cell titles, scope pills, dossier cards) once Phase 148g routed
-- every source name through the emic designation. Capitalise it to 'Franceinfo'
-- to match the sibling pattern. This is a display-only value — the discourse-
-- function classification, weights, and review metadata are unchanged.

UPDATE source_classifications
   SET emic_designation = 'Franceinfo'
 WHERE source_id = (SELECT id FROM sources WHERE name = 'franceinfo')
   AND emic_designation = 'franceinfo';
