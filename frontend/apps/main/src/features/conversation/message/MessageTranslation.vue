<template>
  <div v-if="view.mode === 'show'" class="mt-1 mb-1 border-t border-border/60 pt-1 text-sm">
    <button
      type="button"
      class="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground"
      :aria-expanded="open"
      @click="open = !open"
    >
      <Languages :size="12" />
      <span>{{ label }}</span>
      <ChevronDown :size="12" class="transition-transform" :class="{ 'rotate-180': open }" />
    </button>
    <p v-if="open" class="mt-1 whitespace-pre-wrap break-words" data-testid="message-translation">
      {{ view.translation.text }}
    </p>
  </div>
  <div v-else-if="view.mode === 'offer'" class="mt-1 mb-1">
    <button
      type="button"
      class="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground disabled:opacity-60"
      :disabled="loading"
      @click="translate"
    >
      <Languages :size="12" />
      <span>{{
        loading
          ? t('conversation.translation.translating')
          : t('conversation.translation.translate')
      }}</span>
    </button>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ChevronDown, Languages } from 'lucide-vue-next'
import api from '@main/api'
import { useConversationStore } from '@main/stores/conversation'
import { useEmitter } from '@main/composables/useEmitter'
import { useTranslationLanguage } from '@main/composables/useTranslationLanguage'
import { EMITTER_EVENTS } from '@main/constants/emitterEvents.js'
import { handleHTTPError } from '@shared-ui/utils/http.js'
import { translationView, withTranslation } from './messageTranslation'

const props = defineProps({
  message: { type: Object, required: true }
})

const { t } = useI18n()
const convStore = useConversationStore()
const emitter = useEmitter()
const { translationLanguage } = useTranslationLanguage()
const open = ref(false)
const loading = ref(false)

const view = computed(() => translationView(props.message, translationLanguage.value))

const label = computed(() => {
  const to = (view.value.to || '').toUpperCase()
  if (view.value.isOriginal) return t('conversation.translation.original', { language: to })
  const from = (view.value.from || '?').toUpperCase()
  return t('conversation.translation.label', { from, to })
})

const translate = async () => {
  const language = translationLanguage.value
  if (!language || loading.value) return
  loading.value = true
  try {
    const resp = await api.translateMessage(props.message.conversation_uuid, props.message.uuid, {
      language
    })
    const translation = resp?.data?.data?.translation
    if (translation?.text) {
      convStore.mergeMessageUpdate({
        conversation_uuid: props.message.conversation_uuid,
        uuid: props.message.uuid,
        meta: withTranslation(props.message.meta, language, translation)
      })
      open.value = true
    }
  } catch (error) {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'destructive',
      description: handleHTTPError(error).message
    })
  } finally {
    loading.value = false
  }
}
</script>
