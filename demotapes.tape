Output "./demo/glib-core-loop.gif"

Set Theme "Dracula"
Set FontSize 18
Set Width 1080
Set Height 720
Set Framerate 24
Set TypingSpeed 90ms

# cold start
Type "glib"
Enter
Sleep 2s

# auth already done, let repo list populate
Sleep 1200ms

# filter repo and open
Type "demo"
Sleep 700ms
Enter
Sleep 1500ms

# diff history navigation, open commit
Type "j"
Sleep 250ms
Type "j"
Sleep 250ms
Type "k"
Sleep 350ms
Enter
Sleep 1500ms

# jump files in diff
Type "n"
Sleep 500ms
Type "n"
Sleep 500ms

# go to git, stage one file
Type "g"
Sleep 1400ms
Type "s"
Sleep 1200ms

# go to pi, prompt, show streaming
Type "i"
Sleep 1200ms
Type "summarize the recent changes in this repo"
Enter
Sleep 3500ms

# soft pause to projects with PI active indicator
Escape
Sleep 1500ms
Escape
Sleep 1600ms
Escape
Sleep 900ms

# quit from projects, then hard fallback for stubborn sessions
Type "q"
Sleep 1000ms
Ctrl+C
Sleep 300ms
Ctrl+C
Sleep 300ms
Type "exit"
Enter
Sleep 200ms
