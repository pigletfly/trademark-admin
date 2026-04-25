package export

import "embed"

//go:embed template_zh.html template_en.html template_bilingual.html
var templatesFS embed.FS
