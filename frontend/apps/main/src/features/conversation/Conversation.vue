<template>
  <div class="flex h-full min-h-0 flex-col">
    <!-- Header -->
    <div class="flex h-14 shrink-0 items-center justify-between gap-2 border-b border-border px-3">
      <div class="flex items-center gap-1 min-w-0">
        <Button
          v-if="isMobile"
          variant="ghost"
          class="w-11 h-11 lg:w-8 lg:h-8 p-0 shrink-0 -ml-2 lg:-ml-1"
          :aria-label="t('globals.messages.back')"
          @click="goBackToList"
        >
          <ChevronLeft class="w-4 h-4" />
        </Button>
        <span class="truncate text-sm font-semibold" :title="conversationStore.currentContactName">
          {{ conversationStore.currentContactName }}
        </span>
      </div>
      <div class="flex items-center gap-2 shrink-0">
        <Button
          type="button"
          variant="ghost"
          :class="MOBILE_ICON_BUTTON_CLASS"
          :aria-label="t('copilot.details')"
          :aria-expanded="detailsOpen"
          @click="emitter.emit(EMITTER_EVENTS.CONVERSATION_SIDEBAR_TOGGLE)"
        >
          <PanelRightClose v-if="detailsOpen" class="w-4 h-4" aria-hidden="true" />
          <PanelRight v-else class="w-4 h-4" aria-hidden="true" />
        </Button>
        <Tooltip v-if="isSnoozed && snoozedUntilLabel">
          <TooltipTrigger as-child>
            <span class="flex items-center gap-1 text-xs text-muted-foreground whitespace-nowrap">
              <Clock :size="12" />
              {{ snoozedUntilLabel }}
            </span>
          </TooltipTrigger>
          <TooltipContent>
            {{ t('conversation.snoozedUntil', { time: snoozedUntilLabel }) }}
          </TooltipContent>
        </Tooltip>
        <DropdownMenu>
          <DropdownMenuTrigger>
            <div
              v-if="conversationStore.current?.status"
              class="flex h-11 lg:h-8 items-center cursor-pointer"
            >
              <span
                class="rounded-md bg-primary px-2.5 py-1 text-xs lg:text-sm font-medium text-primary-foreground"
              >
                {{ conversationStore.current?.status }}
              </span>
            </div>
          </DropdownMenuTrigger>
          <DropdownMenuContent>
            <DropdownMenuItem
              v-for="status in conversationStore.statusOptions"
              :key="status.value"
              @click="handleUpdateStatus(status.label)"
            >
              {{ status.label }}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
        <DropdownMenu>
          <DropdownMenuTrigger as-child>
            <Button
              variant="ghost"
              :class="MOBILE_ICON_BUTTON_CLASS"
              :aria-label="t('globals.messages.moreActions')"
            >
              <MoreHorizontal class="w-4 h-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuSub>
              <DropdownMenuSubTrigger>
                {{ t('conversation.downloadTranscript') }}
              </DropdownMenuSubTrigger>
              <DropdownMenuSubContent>
                <DropdownMenuItem @click="downloadTranscript('txt')">
                  {{ t('globals.terms.plainText') }}
                </DropdownMenuItem>
                <DropdownMenuItem @click="downloadTranscript('pdf')">
                  {{ t('globals.terms.pdf') }}
                </DropdownMenuItem>
              </DropdownMenuSubContent>
            </DropdownMenuSub>
            <DropdownMenuItem
              v-if="userStore.can(perms.MESSAGES_WRITE_PRIVATE)"
              :disabled="isSummarizing"
              @click="summarize"
            >
              {{ t('conversation.summarize') }}
            </DropdownMenuItem>
            <DropdownMenuItem
              v-if="userStore.can(perms.CONVERSATIONS_DELETE)"
              class="text-destructive"
              @click="openDeleteDialog"
            >
              {{ t('conversation.delete') }}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </div>

    <AlertDialog :open="showDeleteDialog" @update:open="showDeleteDialog = $event">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{{ t('conversation.delete') }}</AlertDialogTitle>
          <AlertDialogDescription>
            {{
              t('conversation.deleteConfirmation', {
                referenceNumber: conversationStore.current?.reference_number
              })
            }}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <Input
          v-model="deleteConfirmation"
          :placeholder="t('conversation.deleteConfirmationPlaceholder')"
        />
        <AlertDialogFooter>
          <AlertDialogCancel>{{ t('globals.messages.cancel') }}</AlertDialogCancel>
          <AlertDialogAction :disabled="!deleteConfirmed || isDeleting" @click="deleteConversation">
            {{ t('globals.messages.delete') }}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>

    <!-- Messages & reply box -->
    <div class="flex min-h-0 flex-1 flex-col overflow-hidden">
      <MessageList class="min-h-0 flex-1 overflow-y-auto" />
      <WhatsAppComposer v-if="isWhatsAppChannel && canCompose" />
      <ReplyBox v-else-if="canCompose" />
    </div>
  </div>
</template>

<script setup>
const MOBILE_ICON_BUTTON_CLASS = 'w-11 h-11 lg:w-8 lg:h-8 p-0'

import { computed, ref, onMounted, onUnmounted } from 'vue'
import { useConversationStore } from '@main/stores/conversation'
import { useUserStore } from '@main/stores/user'
import { Clock, MoreHorizontal, ChevronLeft, PanelRight, PanelRightClose } from 'lucide-vue-next'
import { useRoute, useRouter } from 'vue-router'
import { useIsMobile } from '@shared-ui/composables'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger
} from '@shared-ui/components/ui/dropdown-menu'
import { Tooltip, TooltipContent, TooltipTrigger } from '@shared-ui/components/ui/tooltip'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle
} from '@shared-ui/components/ui/alert-dialog'
import { Input } from '@shared-ui/components/ui/input'
import { formatMessageTimestamp } from '@shared-ui/utils/datetime.js'
import { Button } from '@shared-ui/components/ui/button'
import MessageList from '@/features/conversation/message/MessageList.vue'
import ReplyBox from './ReplyBox.vue'
import WhatsAppComposer from './WhatsAppComposer.vue'
import { WHATSAPP_CHANNEL } from '@main/features/conversation/whatsappTemplate'
import { EMITTER_EVENTS, CONVERSATION_ACTIONS } from '@main/constants/emitterEvents.js'
import { useCommandPalette } from '@main/features/command/useCommandPalette'
import { SNOOZE_COMMAND } from '@main/features/command/providers/useConversationCommands'
import { CONVERSATION_DEFAULT_STATUSES } from '@main/constants/conversation'
import { useEmitter } from '@main/composables/useEmitter'
import { useI18n } from 'vue-i18n'
import { handleHTTPError } from '@shared-ui/utils/http.js'
import { downloadBlobResponse, parseBlobError } from '@shared-ui/utils/file'
import { printTranscript } from '@main/features/conversation/print-transcript'
import api from '@main/api'
import { permissions as perms } from '@main/constants/permissions.js'
defineProps({ detailsOpen: { type: Boolean, default: false } })
const conversationStore = useConversationStore()
const userStore = useUserStore()
const emitter = useEmitter()
const palette = useCommandPalette()
const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const isMobile = useIsMobile()
const canCompose = computed(
  () => userStore.can(perms.MESSAGES_WRITE) || userStore.can(perms.MESSAGES_WRITE_PRIVATE)
)

// Each detail route is `<list route name>-conversation`.
const goBackToList = () => {
  const listName = String(route.name).replace(/-conversation$/, '')
  const { uuid, ...params } = route.params
  const target = router.resolve({ name: listName, params })
  if (window.history.state?.back?.split('?')[0] === target.path) router.back()
  else router.push(target)
}

const isSnoozed = computed(
  () => conversationStore.current?.status === CONVERSATION_DEFAULT_STATUSES.SNOOZED
)
const snoozedUntilLabel = computed(() =>
  conversationStore.current?.snoozed_until
    ? formatMessageTimestamp(conversationStore.current.snoozed_until)
    : ''
)

const downloadTranscript = async (format = 'txt') => {
  const conversation = conversationStore.current
  if (!conversation) return
  try {
    const response = await api.getConversationTranscript(conversation.uuid)
    const filename = `transcript-${conversation.reference_number}`
    if (format === 'pdf') {
      await printTranscript(await response.data.text(), filename)
    } else {
      downloadBlobResponse(response, `${filename}.txt`)
    }
  } catch (error) {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'destructive',
      description: handleHTTPError(await parseBlobError(error)).message
    })
  }
}

const isWhatsAppChannel = computed(
  () => conversationStore.current?.inbox_channel === WHATSAPP_CHANNEL
)

// Deleting is final, so the agent types the reference number before the button enables.
const showDeleteDialog = ref(false)
const deleteConfirmation = ref('')
const isDeleting = ref(false)
const deleteConfirmed = computed(
  () =>
    deleteConfirmation.value.trim().replace(/^#/, '') ===
    String(conversationStore.current?.reference_number ?? '')
)
const openDeleteDialog = () => {
  deleteConfirmation.value = ''
  showDeleteDialog.value = true
}
const deleteConversation = async () => {
  const conversation = conversationStore.current
  if (!conversation || !deleteConfirmed.value) return
  isDeleting.value = true
  try {
    await api.deleteConversation(conversation.uuid)
    showDeleteDialog.value = false
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, { description: t('conversation.deleted') })
    goBackToList()
    conversationStore.removeConversation(conversation.uuid)
    conversationStore.fetchSidebarCounts({ force: true })
  } catch (error) {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'destructive',
      description: handleHTTPError(error).message
    })
  } finally {
    isDeleting.value = false
  }
}

const isSummarizing = ref(false)

const summarize = async () => {
  const conversation = conversationStore.current
  if (!conversation || isSummarizing.value) return
  try {
    isSummarizing.value = true
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'info',
      description: t('conversation.summarizing')
    })
    await api.aiSummarizeConversation({ conversation_uuid: conversation.uuid })
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      description: t('conversation.summarizeAdded')
    })
  } catch (error) {
    emitter.emit(EMITTER_EVENTS.SHOW_TOAST, {
      variant: 'destructive',
      description: handleHTTPError(error).message
    })
  } finally {
    isSummarizing.value = false
  }
}

const handleUpdateStatus = (status) => {
  if (status === CONVERSATION_DEFAULT_STATUSES.SNOOZED) {
    palette.openPalette({ parent: SNOOZE_COMMAND })
    return
  }
  conversationStore.updateStatus(conversationStore.current.uuid, status)
}

const paletteActions = {
  [CONVERSATION_ACTIONS.DOWNLOAD_TRANSCRIPT]: downloadTranscript,
  [CONVERSATION_ACTIONS.DOWNLOAD_TRANSCRIPT_PDF]: () => downloadTranscript('pdf'),
  [CONVERSATION_ACTIONS.SUMMARIZE]: summarize
}
const onPaletteAction = (action) => paletteActions[action]?.()

onMounted(() => emitter.on(EMITTER_EVENTS.CONVERSATION_ACTION, onPaletteAction))
onUnmounted(() => emitter.off(EMITTER_EVENTS.CONVERSATION_ACTION, onPaletteAction))
</script>
