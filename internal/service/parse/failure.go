package parse

func publicParseErrorMessage(errorCode string) string {
	switch errorCode {
	case "INTERNAL_ERROR":
		return "Skill parse failed. Please retry later."
	case "INVALID_ZIP":
		return "Uploaded file is not a valid ZIP archive."
	case "FILE_TOO_LARGE":
		return "Uploaded file exceeds the size limit."
	case "SKILL_MD_TOO_LARGE":
		return "SKILL.md exceeds the size limit."
	case "SKILL_MD_NOT_FOUND":
		return "SKILL.md was not found in the archive."
	case "ZIP_SLIP_DETECTED":
		return "Archive contains an unsafe file path."
	case "INVALID_SKILL_MD":
		return "SKILL.md is invalid."
	case "SKILL_NAME_MISMATCH":
		return "Uploaded Skill does not match the target Skill."
	case "DUPLICATE_NAME":
		return "A Skill with the same name already exists in this Space."
	case "PARSE_RETRY_EXHAUSTED":
		return "Skill parse timed out after multiple attempts. Please upload again."
	case "PARSE_QUEUE_FULL":
		return "Parse queue is busy. Please retry later."
	default:
		return "Skill parse failed."
	}
}

func publicParseErrorMessageWithDetail(errorCode, _ string) string {
	return publicParseErrorMessage(errorCode)
}
