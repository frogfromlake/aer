-- Revert migration 032: restore franceinfo's lower-case official brand styling.

UPDATE source_classifications
   SET emic_designation = 'franceinfo'
 WHERE source_id = (SELECT id FROM sources WHERE name = 'franceinfo')
   AND emic_designation = 'Franceinfo';
