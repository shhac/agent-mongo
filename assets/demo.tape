# agent-mongo demo recording
# Run: vhs assets/demo.tape
#
# assets/demo-setup.sh puts the demo in a sandbox: a throwaway Docker mongod
# on port 27018, an XDG_CONFIG_HOME under /tmp, and AGENT_MONGO_NO_KEYCHAIN=1.
# No real database, stored connection, or keychain entry is touched.
Output assets/demo.gif
Output assets/demo.webm

Set Theme "Catppuccin Mocha"
Set FontFamily "Fira Code"
Set FontSize 14
Set Width 900
Set Height 560
Set Padding 16
Set Framerate 5
Set TypingSpeed 40ms
Set CursorBlink false

# Hidden setup: sandboxed mongod + isolated config
Hide
Type "bash assets/demo-setup.sh"
Enter
Sleep 25s
Type "export XDG_CONFIG_HOME=/tmp/agent-mongo-demo/config AGENT_MONGO_NO_KEYCHAIN=1"
Enter
Sleep 300ms
Type "clear"
Enter
Sleep 300ms
Show

# What is on this server?
Type "agent-mongo database list"
Sleep 200ms
Enter
Sleep 2.5s

# What is in this database?
Type "agent-mongo collection list shop"
Sleep 200ms
Enter
Sleep 2.5s

# Infer the shape from real documents, before writing a query
Type "agent-mongo collection schema shop users"
Sleep 200ms
Enter
Sleep 3.5s

# Now a query can be written against fields that exist
Type "agent-mongo query find shop users --filter '"
Type '{"active":true}'
Type "' --projection '"
Type '{"name":1,"plan":1,"_id":0}'
Type "'"
Sleep 200ms
Enter
Sleep 3.5s

# Aggregation — $out and $merge are rejected, it is read-only
Type "agent-mongo query aggregate shop orders --pipeline '"
Type '[{"$group":{"_id":"$status","total":{"$sum":"$total"}}}]'
Type "'"
Sleep 200ms
Enter
Sleep 3.5s

# See exactly what ran
Type "agent-mongo query count shop orders --filter '"
Type '{"status":"paid"}'
Type "' --echo-query"
Sleep 200ms
Enter
Sleep 3.5s
