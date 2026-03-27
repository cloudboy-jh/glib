package pi

import "fmt"

type PiEventMsg struct {
	Raw     []byte
	Type    string
	Payload map[string]any
}

type PiResponseMsg struct {
	Raw     []byte
	ID      string
	Command string
	Success bool
	Error   string
	Data    map[string]any
}

type PiExitMsg struct {
	Err error
}

func ResponseFromPayload(raw []byte, payload map[string]any) PiResponseMsg {
	command, _ := payload["command"].(string)
	success, _ := payload["success"].(bool)
	errText, _ := payload["error"].(string)
	id, _ := payload["id"].(string)
	data, _ := payload["data"].(map[string]any)
	return PiResponseMsg{Raw: raw, ID: id, Command: command, Success: success, Error: errText, Data: data}
}

func CmdPrompt(message string) map[string]any {
	return map[string]any{"type": "prompt", "message": message}
}

func CmdPromptWithStreamingBehavior(message, behavior string) map[string]any {
	cmd := map[string]any{"type": "prompt", "message": message}
	if behavior != "" {
		cmd["streamingBehavior"] = behavior
	}
	return cmd
}

func CmdSteer(message string) map[string]any {
	return map[string]any{"type": "steer", "message": message}
}

func CmdFollowUp(message string) map[string]any {
	return map[string]any{"type": "follow_up", "message": message}
}

func CmdAbort() map[string]any {
	return map[string]any{"type": "abort"}
}

func CmdNewSession() map[string]any {
	return map[string]any{"type": "new_session"}
}

func CmdGetState() map[string]any {
	return map[string]any{"type": "get_state"}
}

func CmdCycleModel() map[string]any {
	return map[string]any{"type": "cycle_model"}
}

func CmdSetModel(provider, modelID string) map[string]any {
	return map[string]any{"type": "set_model", "provider": provider, "modelId": modelID}
}

func CmdExtensionUIResponse(id string, payload map[string]any) (map[string]any, error) {
	if id == "" {
		return nil, fmt.Errorf("missing extension ui request id")
	}
	cmd := map[string]any{"type": "extension_ui_response", "id": id}
	for k, v := range payload {
		cmd[k] = v
	}
	return cmd, nil
}
