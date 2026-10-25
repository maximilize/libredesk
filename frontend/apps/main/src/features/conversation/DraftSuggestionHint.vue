<template>
  <div class="px-1 pb-1 text-xs text-muted-foreground">
    <div class="flex items-center gap-1.5">
      <Sparkles class="size-3.5 shrink-0" />
      <span class="truncate">
        {{ $t('conversation.draftSuggestion.label')
        }}<template v-if="author"> · {{ author }}</template>
      </span>
      <template v-if="original">
        <span aria-hidden="true">·</span>
        <button
          type="button"
          class="underline-offset-2 hover:text-foreground hover:underline"
          :aria-expanded="showOriginal"
          data-testid="suggestion-original-toggle"
          @click="showOriginal = !showOriginal"
        >
          {{
            $t('conversation.draftSuggestion.original', {
              language: original.language.toUpperCase()
            })
          }}
        </button>
      </template>
      <span aria-hidden="true">·</span>
      <button
        type="button"
        class="underline-offset-2 hover:text-foreground hover:underline"
        @click="$emit('discard')"
      >
        {{ $t('conversation.draftSuggestion.discard') }}
      </button>
    </div>
    <p
      v-if="original && showOriginal"
      class="mt-1 max-h-40 overflow-y-auto whitespace-pre-wrap break-words rounded-md border border-border bg-muted/40 p-2 text-sm text-foreground"
      data-testid="suggestion-original"
    >
      {{ original.text }}
    </p>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'
import { Sparkles } from 'lucide-vue-next'
import { suggestionAuthorName, suggestionOriginal } from '@main/composables/draftSuggestion'

const props = defineProps({ suggestion: { type: Object, required: true } })
defineEmits(['discard'])

const author = computed(() => suggestionAuthorName(props.suggestion))
const original = computed(() => suggestionOriginal(props.suggestion))
// Open by default: the agent should read what the suggestion says before sending it.
const showOriginal = ref(true)
</script>
