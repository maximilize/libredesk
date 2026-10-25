import { ref } from 'vue'
import api from '@main/api'

// The team's working language for message translations, loaded once per session; '' means translations are off.
const language = ref('')
let loading = null

export function useTranslationLanguage() {
  if (!loading) {
    loading = api
      .getTranslationLanguage()
      .then((resp) => {
        language.value = resp?.data?.data?.language || ''
      })
      .catch(() => {
        loading = null
      })
  }
  return { translationLanguage: language }
}

// setTranslationLanguage updates the cached value after an admin changes it.
export function setTranslationLanguage(value) {
  language.value = value || ''
}
