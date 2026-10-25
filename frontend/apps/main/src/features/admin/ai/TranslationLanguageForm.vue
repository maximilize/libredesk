<template>
  <Card>
    <CardHeader>
      <CardTitle>{{ t('admin.ai.translationLanguage.title') }}</CardTitle>
      <CardDescription>{{ t('admin.ai.translationLanguage.description') }}</CardDescription>
    </CardHeader>
    <CardContent>
      <form class="flex items-end gap-2" novalidate @submit.prevent="save">
        <div class="space-y-2 flex-1 max-w-xs">
          <label for="translation-language" class="text-sm font-medium leading-none">
            {{ t('admin.ai.translationLanguage.label') }}
          </label>
          <Input
            id="translation-language"
            v-model="language"
            name="translation_language"
            type="text"
            placeholder="de"
            autocomplete="off"
          />
        </div>
        <Button type="submit" :disabled="saving">{{ t('globals.messages.save') }}</Button>
      </form>
      <p class="ui-description mt-2">{{ t('admin.ai.translationLanguage.hint') }}</p>
    </CardContent>
  </Card>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Button } from '@shared-ui/components/ui/button/index.js'
import { Input } from '@shared-ui/components/ui/input/index.js'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle
} from '@shared-ui/components/ui/card/index.js'
import { useEmitter } from '@main/composables/useEmitter.js'
import { setTranslationLanguage } from '@main/composables/useTranslationLanguage'
import { EMITTER_EVENTS } from '@main/constants/emitterEvents.js'
import { handleHTTPError } from '@shared-ui/utils/http.js'
import api from '@main/api'

const { t } = useI18n()
const emitter = useEmitter()
const language = ref('')
const saving = ref(false)

onMounted(async () => {
  try {
    const resp = await api.getTranslationLanguage()
    language.value = resp?.data?.data?.language || ''
  } catch (error) {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'destructive',
      description: handleHTTPError(error).message
    })
  }
})

const save = async () => {
  saving.value = true
  try {
    const resp = await api.updateTranslationLanguage({ language: language.value })
    language.value = resp?.data?.data?.language || ''
    setTranslationLanguage(language.value)
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      description: t('globals.messages.savedSuccessfully')
    })
  } catch (error) {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'destructive',
      description: handleHTTPError(error).message
    })
  } finally {
    saving.value = false
  }
}
</script>
