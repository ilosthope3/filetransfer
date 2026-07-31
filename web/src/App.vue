<template>
  <div class="container">
    <div class="content">
      <div v-if="!isLoggedIn">
        <h2>Login</h2>
        <input type="text" v-model="name" placeholder="Name"/>
        <input type="password" v-model="password" placeholder="Password"/>
        <button @click="login">Login</button>
      </div>

      
      <div v-else>
        <form @submit.prevent="handleUpload">
          <input type="file" ref="fileInput" multiple @change="handleFileChange"/>
          <input type="text" v-model="link" placeholder="Paste a link here..."/>
          
        
          <select v-if="deviceNames.length > 0" v-model="receiver">
            <option disabled value="">Select a device</option>
            <option v-for="i in deviceNames" :key="i" :value="i">{{ i }}</option>
          </select>
          
          <button type="submit">Upload</button>
        </form>

      
        <p v-if="message" :style="{ color: messageType === 'error' ? 'red' : 'green' }">
          {{ message }}
        </p>

      
        <h2>Your Files</h2>
        <button @click="fetchFiles">Load Files</button>
        <ul>
          <li v-for="file in files" :key="file.name">{{ file.name }}</li>
        </ul>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'


const isLoggedIn = ref(false)
const name = ref('')
const password = ref('')
const token = ref(localStorage.getItem('device_token') || '')
const files = ref([])
const link = ref('')
const receiver = ref('')
const message = ref('')
const messageType = ref('success')
const deviceNames = ref([])

const fileInput = ref(null)
const selectedFiles = ref([])

/////////////////////////////////////////////////////////////////////////////////

const handleFileChange = () => {
  selectedFiles.value = fileInput.value.files
}

/////////////////////////////////////////////////////////////////////////////////

const login = async () => {
  try {
    const res = await fetch('/api/auth', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ password: password.value, name: name.value })
    })
    const data = await res.json()
    if (data.token) {
      localStorage.setItem('device_token', data.token)
      token.value = data.token
      isLoggedIn.value = true // UI instantly switches to main screen
      message.value = 'Logged in successfully!'
    } else {
      message.value = 'Wrong password'
      messageType.value = 'error'
    }
  } catch (err) {
    message.value = 'Login failed'
    messageType.value = 'error'
  }
}

const fetchFiles = async () => {
  try {
    const res = await fetch('/api/list', {
      headers: { 'Authorization': `Bearer ${token.value}` }
    })
    if (res.ok) {
      files.value = await res.json()
    }
  } catch (err) {
    console.error('Failed to fetch files', err)
  }
}

const handleUpload = async () => {
  if (selectedFiles.value.length === 0 && !link.value) {
    message.value = 'Please select a file or paste a link'
    messageType.value = 'error'
    return
  }

  const formData = new FormData()
  for (let i = 0; i < selectedFiles.value.length; i++) {
    formData.append('file', selectedFiles.value[i])
  }
  formData.append('text', link.value)
  formData.append('receiver', receiver.value)

  try {
    const res = await fetch('/api/upload', {
      method: 'POST',
      headers: { 'Authorization': `Bearer ${token.value}` },
      body: formData
    })
    if (res.ok) {
      message.value = 'Upload successful!'
      messageType.value = 'success'
      selectedFiles.value = []
      link.value = ''
      fileInput.value.value = '' 
    } else {
      message.value = 'Upload failed'
      messageType.value = 'error'
    }
  } catch (err) {
    message.value = 'Upload error'
    messageType.value = 'error'
  }
}

const fetchNames = async () => {
  try {
    const res = await fetch('/api/devices', {
      headers: { 'Authorization': `Bearer ${token.value}` }
    })
    if (res.ok) {
      const data = await res.json()
      deviceNames.value = data.data
    }
  } catch (err) {
    console.error('Failed to fetch device names', err)
  }
}

///////////////////////////////////////////////////////////////////////////////////

onMounted(async () => {
  if (token.value) {
    try {
      const res = await fetch('/api/whoami', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ Token: token.value })
      })
      if (res.ok) {
        isLoggedIn.value = true
        fetchNames()

      } else {
        localStorage.removeItem('device_token')
        token.value = ''
        isLoggedIn.value = false
      }
    } catch {
      localStorage.removeItem('device_token')
      token.value = ''
      isLoggedIn.value = false
    }
  }
})

</script>

<style scoped>
form {
  background: white;
  padding: 20px;
  border-radius: 10px;
  box-shadow: 0 2px 10px rgba(0,0,0,0.1);
}
input, select, button {
  font-size: 18px;
  padding: 14px;
  width: 100%;
  margin-bottom: 12px;
  box-sizing: border-box;
  border: 1px solid #ccc;
  border-radius: 6px;
  display: block;
}
button {
  background: #007bff;
  color: white;
  border: none;
  font-weight: bold;
  cursor: pointer;
}
button:hover {
  background: #0056b3;
}

.container {
  background-color: #eee;
  width: 100vw;
  height: 100vh;
  display: flex;
  flex-direction: column;
  align-items: center;
  /* justify-content: center; */
  padding-top: 100px;
}

.content {
  width: 400px;
}

</style>