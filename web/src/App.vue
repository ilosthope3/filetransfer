<template>
  <div class="container">
    <div class="content">
      <div v-if="!isLoggedIn" class="loginDiv">
        <input class="loginInput" type="text" v-model="name" placeholder="Name"/>
        <input class="loginInput" type="password" v-model="password" placeholder="Password"/>
        <button class="loginBtn" @click="login">Login</button>
      </div>

      
      <div v-else class="appContent">
        <div class="block">
          <form @submit.prevent="handleUpload">
            <input type="file" ref="fileInput" multiple @change="handleFileChange"/>
            <input type="text" v-model="link" placeholder="Paste a link here..."/>
            
          
            <select v-if="deviceNames.length > 0" v-model="receiver">
              <option disabled value="">Select a device</option>
              <option v-for="i in deviceNames" :key="i" :value="i">{{ i }}</option>
            </select>

            <p v-else>No devices found</p>
        
            

            <button type="submit">Upload</button>
          </form>
          <p v-if="message" :style="{ color: messageType === 'error' ? 'red' : 'green' }">
            {{ message }}
          </p>

          <button @click="fetchNames">Refresh</button>
        </div>
      
        
        <div class="block">
          <div class="fileHeader"> 
            <h2>Your Files</h2>
            <button @click="fetchFiles">Refresh</button>
          </div>
          <ul class="fileList">
            <li class="fileItem" v-for="file in files" :key="file.name">{{ file.filename }} - {{file.timestamp}}</li>
          </ul>
        </div>
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
      const res = await fetch('/api/files', {
        headers: { 'Authorization': `Bearer ${token.value}` }
      })
      console.log(res)
      if (res.ok) {
        const data = await res.json()
        
        console.log(data, '111111111111111')
        files.value = data.data
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
      console.log('1')
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
          fetchFiles()

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
  
  input, select, button {
    
  }
  button {
    
  }
  button:hover {
   
  }

  .container {
    
    width: 100vw;
    height: 100vh;
    display: flex;
    flex-direction: column;
    align-items: center;
    background-color: var(--color-accent-light3);
    padding: 50px;
  }

  .content {
    width: 70vw;
    display: flex;
    flex-direction: row;
    gap: 25px;
  }

  .loginDiv {
    width: 100%;
    display: flex;
    flex-direction: column;
    gap: 10px;
    justify-content: center;
    align-items: center;
  }

  .loginInput {
    box-shadow: none;
    height: 8vh;
    border-radius: 50vh;
    border-color: var(--color-accent-light);
    border-width: 3px;
    width: 100%;
    transition: all 0.15s ease;
    padding: 0px 20px;
    font-size: 1rem;
  }

  .loginInput:hover {
    border-color: var(--color-accent-light);
    border-width: 8px;
  } 

  .loginInput:focus {
    outline: none;
    border-color: var(--color-accent-light);
    border-width: 2px;
    
    border-left-width: 10px;
    border-right-width: 10px;
    
    border-radius: 25vh;
  }

  .loginBtn {
    width: 50%;
    border-radius: 50vh;
    height: 6vh;
    background-color: var(--color-accent-light1);
    /* transition: all 0.15s ease; */
    transition: all 0.18s ease;
    font-size: 1rem;
    border-color: var(--color-border);
    color: var(--color-heading)
  }

  .loginBtn:hover {
    color: var(--color-accent-dark);
    border-width: 5px;
    background-color: var(--color-accent-light2);
    border-color: var(--color-accent-light);
    
  } 

  .loginBtn:active {
    background-color: var(--color-accent-light);
    transition-duration: 0.05s;
    border: none;
    box-shadow: none;

  }


  .block {
    background-color: var(--bgd);
    display: flex;
    flex-direction: column;
    width: auto;
    align-items: center;
    border-width: 2px;
    border-color: var(--border);
    border-style: solid;
    border-radius: 20px;
    padding: 15px;
    box-shadow: 0px 0px 10px var(--border)
  }

  .fileHeader {
    display: flex;
    position: relative;
    min-width: 500px;
    align-items: center;
    justify-content: center;
    gap: 10px;
    padding-bottom: 10px;
    margin-bottom: 20px;
  }

  .fileHeader::after {
    width: 80%;
    position: absolute;
    min-height: 1px;
    background-color: var(--border);
    bottom: 0;
    left: 10%;
    content: '';
  }

  .fileList {
    width: 100%;
    /* background-color: red; */
    display: flex;
    flex-direction: column;
    gap: 10px;
    list-style: none;
  }

  .fileItem {
    padding: 5px;
  }

  .appContent {
    display: flex;
    flex-direction: row;
    width: 100%;
    justify-content: center;
    gap: 20px;
  }




</style>