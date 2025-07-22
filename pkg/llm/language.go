package llm

func GetLanguageName(lang string) string {
	switch lang {
	case "en":
		return "英文"
	case "ch":
		return "中文"
	default:
		return "英文"
	}
}
