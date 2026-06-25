import { Globe } from "lucide-react"
import { useTranslation } from "react-i18next"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"

const LANGUAGES = [
  { code: "en", label: "English" },
  { code: "zh", label: "中文" },
  { code: "fr", label: "Français" },
  { code: "ru", label: "Русский" },
] as const

export function LanguageSwitcher() {
  const { i18n } = useTranslation()

  return (
    <Select value={i18n.language} onValueChange={(lang) => i18n.changeLanguage(lang)}>
      <SelectTrigger data-testid="language-switcher" className="w-auto gap-2">
        <Globe className="size-4" />
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {LANGUAGES.map(({ code, label }) => (
          <SelectItem key={code} value={code}>
            {label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}
