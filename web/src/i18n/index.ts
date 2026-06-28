import i18n from "i18next"
import { initReactI18next } from "react-i18next"
import en from "./en.json"
import zh from "./zh.json"
import fr from "./fr.json"
import ru from "./ru.json"

// getAppLang reads the server-injected language from the <meta name="app-lang">
// tag. Falls back to "en" when the tag is absent or empty.
export function getAppLang(): string {
  const meta = document.querySelector<HTMLMetaElement>('meta[name="app-lang"]')
  const lang = meta?.content ?? ""
  return lang || "en"
}

i18n.use(initReactI18next).init({
  resources: {
    en: { translation: en },
    zh: { translation: zh },
    fr: { translation: fr },
    ru: { translation: ru },
  },
  lng: getAppLang(),
  fallbackLng: "en",
  interpolation: {
    escapeValue: false,
  },
})

export default i18n
