<script setup lang="ts">
import { computed } from 'vue'
import Icon from '../ui/Icon.vue'
import { useCollectionsStore } from '../../stores/collections'
import { formatAgo, useMessages } from '../../i18n'

// Where the request on screen sits, and when it was last written. The path stands here rather than in
// the status bar because this is the surface the request is open on: the line above the bar says what
// the pane below is a picture of, and the bar at the bottom of the window is about the window.
const collections = useCollectionsStore()
const { t } = useMessages()

// The time comes from the node the card opened, which Go reads whole — the tree deliberately carries
// no request payload, and a timestamp is not what a row of names is for.
const edited = computed(() => {
  const at = collections.editor?.node.updatedAt ?? 0
  return at > 0 ? t('collections.edited', { ago: formatAgo(at) }) : ''
})
</script>

<template>
  <div class="crumbs">
    <Icon name="folder" :size="15" :stroke-width="1.7" class="crumb-folder" />
    <template v-for="(crumb, i) in collections.breadcrumbs" :key="crumb.id">
      <Icon
        v-if="i > 0"
        name="chevron-right"
        :size="12"
        :stroke-width="2.4"
        class="crumb-sep"
      />
      <span class="crumb" :class="{ 'crumb-last': i === collections.breadcrumbs.length - 1 }">
        {{ crumb.name }}
      </span>
    </template>
    <span class="spacer"></span>
    <span v-if="edited" class="edited">{{ edited }}</span>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.crumbs {
  @apply flex-none flex items-center gap-2 h-11 px-5 border-b border-border
         text-[13px] text-text-tertiary;
}

/* The one accent in the line: which collection the request belongs to is the part of the path that
   says where the request lives, and the rest is the way there. */
.crumb-folder {
  @apply flex-none text-accent;
}

.crumb-sep {
  @apply flex-none;
}

.crumb {
  @apply min-w-0 overflow-hidden text-ellipsis whitespace-nowrap;
}

/* The request itself is what is open, so it is the one crumb written in full ink. */
.crumb-last {
  @apply text-text font-medium;
}

.spacer {
  @apply flex-1;
}

.edited {
  @apply flex-none text-[12.5px];
}
</style>
