@echo off
:: Change directory to the script's location (where the .exe should be)
cd /D "%~dp0"

:: Start the application without a console window
:: The "" is a dummy title required by start /B
start "" /B "ClipboardRegexReplace.exe"

exit