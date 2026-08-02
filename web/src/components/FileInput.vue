<template>
  <div class="drop-zone"
    @dragover.prevent="onDragOver"
    @dragenter.prevent="onDragEnter"
    @dragleave.prevent="onDragLeave"
    @drop.prevent="onDrop"
    @click="triggerFileInput"
    :class="{ 'drop-zone--dragover': isDragging }"
  >
    <input type="file"
        ref="fileInputRef"
        multiple
        @change="onFileSelected"
        style="display: none"
    />

    <div class="drop-zone__content">
      <svg class="drop-zone__icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M12 4v12m0-12l-3 3m3-3l3 3M4 16v2a2 2 0 002 2h12a2 2 0 002-2v-2" stroke-linecap="round" stroke-linejoin="round"/>
      </svg>
      <p class="drop-zone__text">
        <span class="drop-zone__bold">Drag & drop files here</span><br>
        or click to browse
      </p>
      <p class="drop-zone__file-list">
        {{ selectedFiles.length }} selected
      </p>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'

const isDragging = ref(false)
const selectedFiles = ref([])
const fileInputRef = ref(null)

const emit = defineEmits(['files-selected'])

const triggerFileInput = () => {
  fileInputRef.value.click()
}

const onDragOver = (e) => {
  
}

const onDragEnter = (e) => {
  isDragging.value = true
}

const onDragLeave = (e) => {
  if (!e.relatedTarget || !e.currentTarget.contains(e.relatedTarget)) {
    isDragging.value = false
  }
}

const onDrop = (e) => {
  isDragging.value = false
  const files = e.dataTransfer.files
  if (files.length) {
    selectedFiles.value = Array.from(files)
    emit('files-selected', selectedFiles.value) 
  }
}

const onFileSelected = (e) => {
  const files = e.target.files
  if (files.length) {
    selectedFiles.value = Array.from(files)
    emit('files-selected', selectedFiles.value) 
  }
  e.target.value = ''
}

const clearFiles = () => {
  selectedFiles.value = []  
  if (fileInputRef.value) {
    fileInputRef.value.value = '' 
  }
}

defineExpose({
  clearFiles
})

</script>

<style scoped>
.drop-zone {
  border: 2px dashed #ccc;
  border-radius: 12px;
  padding: 40px 20px;
  text-align: center;
  cursor: pointer;
  transition: background-color 0.2s, border-color 0.2s, box-shadow 0.2s;
  background-color: #fafafa;
  min-height: 200px;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  color: #555
}

.drop-zone:hover {
  border-color: var(--border);
  background-color: #ffeeee;
  color: var(--border)
}

.drop-zone--dragover {
  border-color: var(--border);
  background-color: #ffeeee;
  box-shadow: inset 0 2px 8px rgba(192, 22, 22, 0.15);
}

.drop-zone__content {
  pointer-events: none;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.drop-zone__icon {
  width: 48px;
  height: 48px;
  color: #888;
  transition: color 0.2s;
}

.drop-zone--dragover .drop-zone__icon,
.drop-zone:hover .drop-zone__icon {
  color: var(--border);
}

.drop-zone__text {
  font-size: 16px;
  color: inherit;
  margin: 0;
  line-height: 1.6;
  transition: all 0.2s ease;
}

.drop-zone__bold {
  transition: all 0.2s ease;
  font-weight: 600;
  color: inherit;
}

.drop-zone__file-list {
  font-size: 14px;
  color: var(--border);
  font-weight: 500;
  margin: 8px 0 0;
}

/* Responsive */
@media (max-width: 480px) {
  .drop-zone {
    padding: 24px 16px;
    min-height: 150px;
  }
  .drop-zone__icon {
    width: 36px;
    height: 36px;
  }
  .drop-zone__text {
    font-size: 14px;
  }
}
</style>