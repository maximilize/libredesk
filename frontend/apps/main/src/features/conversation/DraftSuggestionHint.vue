<template>
  <div class="flex items-center gap-1.5 px-1 pb-1 text-xs text-muted-foreground">
    <Sparkles class="size-3.5 shrink-0" />
    <span class="truncate">
      {{ $t('conversation.draftSuggestion.label') }}<template v-if="author"> · {{ author }}</template>
    </span>
    <span aria-hidden="true">·</span>
    <button
      type="button"
      class="underline-offset-2 hover:text-foreground hover:underline"
      @click="$emit('discard')"
    >
      {{ $t('conversation.draftSuggestion.discard') }}
    </button>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { Sparkles } from 'lucide-vue-next'
import { suggestionAuthorName } from '@main/composables/draftSuggestion'

const props = defineProps({ suggestion: { type: Object, required: true } })
defineEmits(['discard'])

const author = computed(() => suggestionAuthorName(props.suggestion))
</script>
