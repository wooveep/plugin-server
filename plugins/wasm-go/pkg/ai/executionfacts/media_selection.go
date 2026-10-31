package executionfacts

import "errors"

// decodeMediaSelection implements the existing D0 media selection contract for
// the registered single HTTP sender, independently of text fact registration.
func decodeMediaSelection(raw []byte) (map[string]interface{}, error) {
	object, err := decodeSelectionFields(raw, "operation_id dispatch_id invocation_id gateway_boot_id parent_attempt_id context_generation recovery_claim_refs")
	if err != nil {
		return nil, err
	}
	if object["selection_canonicalization"] != "selection-json.v1" || object["send_transport"] != "http_call" || object["recording_owner"] != "ai_proxy" {
		return nil, errors.New("media selection requires its original single HTTP sender")
	}
	if _, exists := object["operation_id"]; !exists {
		return nil, errors.New("media selection requires its original operation")
	}
	switch object["operation_kind"] {
	case "image_generate", "video_generate":
		if object["mechanism"] != "router" {
			return nil, errors.New("media creation cannot use a retry mechanism")
		}
		for _, field := range []string{"dispatch_id", "invocation_id", "gateway_boot_id", "context_generation"} {
			if _, exists := object[field]; !exists {
				return nil, errors.New("media creation requires its original dispatch and invocation")
			}
		}
	case "task_query":
		if object["mechanism"] != "task_query" || object["dispatch_id"] != nil {
			return nil, errors.New("task query cannot use a create selection")
		}
	default:
		return nil, errors.New("media selection requires a registered media operation")
	}
	return object, nil
}

// MediaSelectionDigest covers the complete original D0 media selection except
// its digest, including the operation, actual invocation and recovery claims.
func MediaSelectionDigest(raw []byte) (string, error) {
	return selectionDigest(raw, decodeMediaSelection)
}

func CanonicalMediaSelection(raw []byte) ([]byte, error) {
	return canonicalSelection(raw, decodeMediaSelection)
}
