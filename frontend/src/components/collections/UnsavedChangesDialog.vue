<script setup lang="ts">
import Alert from '../ui/alert/Alert.vue'
import { Button } from '../ui/button'
import { useCollectionsStore } from '../../stores/collections'
import { useMessages } from '../../i18n'

// The alert that stands between a click and a card with unsaved edits. It is drawn once, at the top
// of the window, because what asks is not always the tree: the rail and the window's close ask too.
const store = useCollectionsStore()

const { t } = useMessages()
</script>

<template>
  <Alert
    :open="store.pendingLeave !== null"
    icon="bookmark"
    :title="t('collections.unsavedTitle', { name: store.pendingLeave?.name })"
    :hint="t('collections.unsavedHint')"
    @cancel="store.answerUnsaved('cancel')"
  >
    <!-- «Не сохранять» stands apart on the left: the one button that throws something away is not
         the one a hand reaches for by accident. -->
    <Button class="text-red" @click="store.answerUnsaved('discard')">{{ t('common.discard') }}</Button>
    <span class="flex-1" />
    <Button @click="store.answerUnsaved('cancel')">{{ t('common.cancel') }}</Button>
    <Button variant="primary" @click="store.answerUnsaved('save')">{{ t('common.save') }}</Button>
  </Alert>
</template>
