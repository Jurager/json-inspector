<script setup lang="ts">
import Alert from '../ui/alert/Alert.vue'
import { Button } from '../ui/button'
import { useRequestsStore } from '../../stores/requests'
import { useMessages } from '../../i18n'

// The same question for the command line: the record about to be opened replaces what is being
// composed. There is no third answer here, unlike a card's — saving means choosing a collection,
// and the line has a button of its own for that beside the address, which is where the hint sends
// whoever wants to keep the work.
const store = useRequestsStore()

const { t } = useMessages()
</script>

<template>
  <Alert
    :open="store.pendingOpen !== null"
    icon="bookmark"
    :title="t('request.unsavedTitle')"
    :hint="t('request.unsavedHint')"
    @cancel="store.answerUnsaved('cancel')"
  >
    <Button class="text-red" @click="store.answerUnsaved('discard')">{{ t('common.discard') }}</Button>
    <span class="flex-1" />
    <Button variant="primary" @click="store.answerUnsaved('cancel')">{{ t('common.cancel') }}</Button>
  </Alert>
</template>
