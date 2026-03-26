| Aktion | Offizieller PS-Befehl | Kurzbefehl / Alias | Beispiel |
| :--- | :--- | :--- | :--- |
| Datei erstellen | New-Item | ni | ni script.js |
| Ordner erstellen | New-Item -ItemType Directory | mkdir, md | mkdir MeinProjekt |
| Datei lesen | Get-Content | cat, type | cat index.html |
| Datei löschen | Remove-Item | rm, del | rm unnoetig.txt |
| Ordner löschen (mit Inhalt) | Remove-Item -Recurse -Force | rm -r -force | rm -r node_modules |
| Datei kopieren | Copy-Item | cp, copy | cp style.css backup.css |
| Datei verschieben | Move-Item | mv, move | mv app.js ./src/ |
| Datei umbenennen | Rename-Item | ren | ren alt.txt neu.txt |
| Inhalt auflisten | Get-ChildItem | ls, dir | ls |
| Ordner wechseln | Set-Location | cd | cd ./src/components/ |