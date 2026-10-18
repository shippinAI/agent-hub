// Package toolcaps is the dependency-free table of bridge tool names and the capability each needs,
// so the hub can authorize a tool call without importing the tool implementations.
// tools.TestCapsTable keeps it in step with tools.All().
package toolcaps

// Cap maps tool name -> capability ("" = none beyond a live grant).
var Cap = map[string]string{
	"get_config": "", "set_config_value": "",
	"read_file": "fs.read", "read_multiple_files": "fs.read", "write_file": "fs.write", "write_pdf": "fs.write",
	"create_directory": "fs.write", "list_directory": "fs.read", "move_file": "fs.write", "get_file_info": "fs.read",
	"start_search": "fs.read", "get_more_search_results": "fs.read", "stop_search": "fs.read", "list_searches": "fs.read",
	"edit_block": "fs.write", "delete_file": "fs.write",
	"start_process": "proc.interactive", "read_process_output": "proc.interactive", "interact_with_process": "proc.interactive",
	"force_terminate": "proc.interactive", "list_sessions": "proc.interactive", "list_processes": "proc.list",
	"kill_process": "proc.kill", "run_command": "exec", "run_code": "exec",
	"get_usage_stats": "", "get_recent_tool_calls": "", "give_feedback_to_desktop_commander": "",
}
