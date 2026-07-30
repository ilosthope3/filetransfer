<template>
  <h1>Hello from Vue</h1>
  <button @click="fetchFiles">Load Files</button>
  <div v-for="f in files">{{ f.name }}</div>
</template>

<script setup>
import { ref } from 'vue'
const files = ref([])

const fetchFiles = async () => {

  console.log('`111111111')
  // const res = await fetch('/api/list')
  // files.value = await res.json()
}
</script>