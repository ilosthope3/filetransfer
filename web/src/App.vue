










<template>
  <div class="container">
    


    <div v-if="!isLoggedIn" class="loginContainer">

      <div class="login">
        <p class="loginMsg">{{loginMsg}}</p>
        <div class="loginLeft">
          <input class="loginInput" type="text" v-model="name" placeholder=""/>
          <input class="loginInput" type="password" v-model="password" placeholder=""/>
        </div>
        <div class="loginRight">
          <button class="loginBtn" @click="login"></button>
        </div>
      </div>
    </div>

    
    <div v-else class="appContent">
      <div class="block">
        <form class="form" @submit.prevent="handleUpload">
          <p class="loginMsg">
            {{ message }}
          </p>
          
          <FileDropZone ref="dropZoneRef" @files-selected="handleFileChange" />
          
          
          <input class="loginInput" type="text" v-model="link" placeholder=""/>
          

          <div class="fieldContainer">
            <div class="bottomLeft">
              <select class="deviceSelect" v-if="deviceNames.length > 0" v-model="receiver">
                <option disabled value="">Select a device</option>
                <option v-for="i in deviceNames" :key="i" :value="i">{{ i }}</option>
              </select>

              <p v-else>No devices found</p>
              <button class="refreshBtn" @click.prevent="fetchNames">

                <svg fill="#ffffff" class="refreshSvg" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" version="1.1" id="Capa_1" x="0px" y="0px" viewBox="0 0 512 512" style="enable-background:new 0 0 512 512;" xml:space="preserve" width="512" height="512">                 
                  <path d="M489.797,256c-10.791-0.141-19.924,7.939-21.099,18.667c-9.959,117.754-113.491,205.138-231.245,195.179   S32.315,356.354,42.275,238.6S155.766,33.462,273.52,43.421c50.983,4.312,98.733,26.75,134.592,63.245h-66.603   c-11.782,0-21.333,9.551-21.333,21.333s9.551,21.333,21.333,21.333h88.384c21.874-0.012,39.604-17.742,39.616-39.616V21.333   C469.509,9.551,459.958,0,448.176,0c-11.782,0-21.333,9.551-21.333,21.333v44.331C321.548-28.425,159.915-19.341,65.826,85.954   s-85.005,266.927,20.29,361.016s266.927,85.005,361.016-20.29c36.575-40.931,59.007-92.547,63.977-147.214   c1.096-11.814-7.593-22.279-19.407-23.375C491.069,256.033,490.434,256.002,489.797,256z"/>
                </svg>
              </button>
            </div>
            <button class="uploadBtn" type="submit">
              <!-- <svg class="uploadSvg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                <path d="M12 4v12m0-12l-3 3m3-3l3 3M4 16v2a2 2 0 002 2h12a2 2 0 002-2v-2" stroke-linecap="round" stroke-linejoin="round"/>
              </svg> -->

              <svg class="uploadSvg" viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
                <path d="M 3.93 5.93 A 10 10 0 0 0 2 12" stroke-dasharray="0 3"/>
                <path d="M 22 12 A 10 10 0 0 0 19.07 4.93" stroke-dasharray="0 3"/>
                <path d="M 22 12 A 10 10 0 0 1 2 12"  />
                <polyline points="12 16 12 8 8 12" />
                <line x1="12" y1="8" x2="16" y2="12" />
              </svg>
            </button>
          </div>
          
          
        </form>
        

        
      </div>
    
      
      <div v-if="files.length != 0" class="block">
        <button class="refreshBtn refreshBtn1" @click="fetchFiles">
          <svg fill="#ffffff" class="refreshSvg" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" version="1.1" id="Capa_1" x="0px" y="0px" viewBox="0 0 512 512" style="enable-background:new 0 0 512 512;" xml:space="preserve" width="512" height="512">                 
            <path d="M489.797,256c-10.791-0.141-19.924,7.939-21.099,18.667c-9.959,117.754-113.491,205.138-231.245,195.179   S32.315,356.354,42.275,238.6S155.766,33.462,273.52,43.421c50.983,4.312,98.733,26.75,134.592,63.245h-66.603   c-11.782,0-21.333,9.551-21.333,21.333s9.551,21.333,21.333,21.333h88.384c21.874-0.012,39.604-17.742,39.616-39.616V21.333   C469.509,9.551,459.958,0,448.176,0c-11.782,0-21.333,9.551-21.333,21.333v44.331C321.548-28.425,159.915-19.341,65.826,85.954   s-85.005,266.927,20.29,361.016s266.927,85.005,361.016-20.29c36.575-40.931,59.007-92.547,63.977-147.214   c1.096-11.814-7.593-22.279-19.407-23.375C491.069,256.033,490.434,256.002,489.797,256z"/>
          </svg>
        </button>
        <button v-if="files.length > 1" class="delete deleteAll" @click="handleDelete('all', true)">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M18 6L6 18" />
            <path d="M6 6L18 18" />
          </svg>
        </button>
        <div class="fileHeader">
          <div class="wrapper">
            <h2>Files</h2>
            <button v-if="files.length > 1" class="delete downloadAll" @click="syncAll">
              <svg viewBox="0 0 24 24" fill="white">
                <path d="M19 9h-4V3H9v6H5l7 7z" />
                <path d="M5 18v2h14v-2z" />
              </svg>
            </button>
            
          </div>
        </div>
        <ul class="fileList">
          <li class="fileItem" v-for="file in files" :key="file.name">
            <a @click="downloadAndDelete(file.id)" class="fileLink">
              {{ file.filename }}
            </a>
            
            <div class="fileInfo">
              <p class="sender">{{file.sender}}</p>
              <p class="date">{{ file.timestamp.slice(5)}}</p>
              <p class="size">
                {{ formatSize(file.size) }}
              </p>
              <button class="delete" @click="handleDelete(file.id, true)">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M18 6L6 18" />
                  <path d="M6 6L18 18" />
                </svg>
              </button>
              
            </div>
          </li>
        </ul>
      </div>

      <!-- <div v-if="links.length != 0" class="block">
        <button class="refreshBtn refreshBtn1" @click="fetchFiles">
          <svg fill="#ffffff" class="refreshSvg" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" version="1.1" id="Capa_1" x="0px" y="0px" viewBox="0 0 512 512" style="enable-background:new 0 0 512 512;" xml:space="preserve" width="512" height="512">                 
            <path d="M489.797,256c-10.791-0.141-19.924,7.939-21.099,18.667c-9.959,117.754-113.491,205.138-231.245,195.179   S32.315,356.354,42.275,238.6S155.766,33.462,273.52,43.421c50.983,4.312,98.733,26.75,134.592,63.245h-66.603   c-11.782,0-21.333,9.551-21.333,21.333s9.551,21.333,21.333,21.333h88.384c21.874-0.012,39.604-17.742,39.616-39.616V21.333   C469.509,9.551,459.958,0,448.176,0c-11.782,0-21.333,9.551-21.333,21.333v44.331C321.548-28.425,159.915-19.341,65.826,85.954   s-85.005,266.927,20.29,361.016s266.927,85.005,361.016-20.29c36.575-40.931,59.007-92.547,63.977-147.214   c1.096-11.814-7.593-22.279-19.407-23.375C491.069,256.033,490.434,256.002,489.797,256z"/>
          </svg>
        </button>
        <div class="fileHeader">
          <div class="wrapper">
            <h2>Your Links</h2>
            
          </div>
        </div>
        <ul class="fileList">
          <li class="fileItem" v-for="link in links" :key="link.url">
            {{link.url}}
          </li>
        </ul>
      </div> -->

      <div v-if="links.length != 0" class="block">
        <button class="refreshBtn refreshBtn1" @click="fetchFiles">
          <svg fill="#ffffff" class="refreshSvg" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" version="1.1" id="Capa_1" x="0px" y="0px" viewBox="0 0 512 512" style="enable-background:new 0 0 512 512;" xml:space="preserve" width="512" height="512">                 
            <path d="M489.797,256c-10.791-0.141-19.924,7.939-21.099,18.667c-9.959,117.754-113.491,205.138-231.245,195.179   S32.315,356.354,42.275,238.6S155.766,33.462,273.52,43.421c50.983,4.312,98.733,26.75,134.592,63.245h-66.603   c-11.782,0-21.333,9.551-21.333,21.333s9.551,21.333,21.333,21.333h88.384c21.874-0.012,39.604-17.742,39.616-39.616V21.333   C469.509,9.551,459.958,0,448.176,0c-11.782,0-21.333,9.551-21.333,21.333v44.331C321.548-28.425,159.915-19.341,65.826,85.954   s-85.005,266.927,20.29,361.016s266.927,85.005,361.016-20.29c36.575-40.931,59.007-92.547,63.977-147.214   c1.096-11.814-7.593-22.279-19.407-23.375C491.069,256.033,490.434,256.002,489.797,256z"/>
          </svg>
        </button>
        <button v-if="links.length > 1" class="delete deleteAll" @click="handleDelete('all', false)">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M18 6L6 18" />
            <path d="M6 6L18 18" />
          </svg>
        </button>
        <div class="fileHeader">
          <div class="wrapper">
            <h2>Links</h2>
          </div>
        </div>
        <ul class="fileList">
          <li class="fileItem" v-for="link in links" :key="link.id">
            <a @click="copyToClipboard(link.url, link.id)" class="fileLink">
              {{ link.url }}
            </a>
            
            <div class="fileInfo">
              <p class="sender">{{link.sender}}</p>
              <p class="date">{{ link.timestamp.slice(5)}}</p>
              
              <button class="delete" @click="handleDelete(link.id, false)">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M18 6L6 18" />
                  <path d="M6 6L18 18" />
                </svg>
              </button>
              
            </div>
          </li>
        </ul>
      </div>
    </div>
    
  </div>
</template>

<script setup>
  import { ref, onMounted } from 'vue'
  import FileDropZone from './components/FileInput.vue'

  const isLoggedIn = ref(false)
  const loginMsg = ref('')
  const name = ref('')
  const password = ref('')
  const token = ref(localStorage.getItem('device_token') || '')
  const files = ref([])
  const links = ref([])
  const link = ref('')
  const receiver = ref('')
  const message = ref('')
  const deviceNames = ref([])
  const dropZoneRef = ref(null)
  const inputFiles = ref([])
  const isSyncing = ref(false)
  /////////////////////////////////////////////////////////////////////////////////

  const handleFileChange = async (files) => {
    inputFiles.value = files
  }

  const handleDelete = async (id, isFile) => {
    console.log(id)
    try {
      const res = await fetch(`/api/delete?id=${id}&isFile=${isFile}`, {
          headers: { 'Authorization': `Bearer ${token.value}` }
        })
      if (res.ok) {
        const data = await res.json()
        if (data.success) {
          fetchFiles()
        } else {
          console.log(data.error)
        }
      } else {
        console.log('Unknown error: ', res.status)
      }
    } catch (err) {
      console.log('JSError: ',err)
    }
  }

  const login = async () => {
    loginMsg.value = ""
    if (name.value.length == 0) {
      loginMsg.value = 'Empty name'
      return
    }
    try {
      const res = await fetch('/api/auth', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ password: password.value, name: name.value })
      })
      if (res.status == 502) {
        loginMsg.value = "Couldn't connect to server"
        return
      }
      const data = await res.json()
      if (data.success) {
        localStorage.setItem('device_token', data.data)
        token.value = data.data
        isLoggedIn.value = true 
      } else {
        loginMsg.value = data.error
      }
    } catch (err) {
      loginMsg.value = 'JS Err: ' + err
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
        
        links.value = data.data.links
        files.value = data.data.files
      }
    } catch (err) {
      console.error('Failed to fetch files', err)
    }
  }

  const handleUpload = async () => {

    message.value = ""

    if (inputFiles.value.length === 0 && !link.value) {
      message.value = 'Select a file or paste a link'
      return
    }

    const formData = new FormData()
    for (let i = 0; i < inputFiles.value.length; i++) {
      formData.append('file', inputFiles.value[i])
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
        inputFiles.value = []
        link.value = ''
        
        if (dropZoneRef.value) {
          dropZoneRef.value.clearFiles()
        }
      } else {
        let data = await res.json()
        message.value = data.error
      }
    } catch (err) {
      message.value = 'Upload error' + err
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
        fetchFiles()
      }
    } catch (err) {
      console.error('Failed to fetch device names', err)
    }
  }

  const formatSize = (bytes) => {
    if (bytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    const size = (bytes / Math.pow(k, i)).toFixed(1)
    return size + ' ' + sizes[i]
  }

  const copyToClipboard = async (text, id) => {
    try {
      await navigator.clipboard.writeText(text)
      alert('Link copied to clipboard!')
      handleDelete(id, false)
    } catch (err) {
      console.error('Failed to copy:', err)
      alert('Failed to copy. Please copy manually.')
    }
  }
  
// Download + Delete (single file)
  const downloadAndDelete = async (id) => {
    try {
      const res = await fetch(`/api/download-and-delete?id=${id}`, {
        headers: { 'Authorization': `Bearer ${token.value}` }
      })

      if (!res.ok) {
        const data = await res.json()
        alert(data.error || 'Failed')
        return
      }

      // Trigger browser download
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      const cd = res.headers.get('Content-Disposition')
      const filename = cd ? cd.match(/filename="(.+)"/)[1] : 'downloaded'
      a.download = filename
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      URL.revokeObjectURL(url)

      // Remove from local list (since it's deleted on server)
      files.value = files.value.filter(f => f.id !== id)

    } catch (err) {
      console.error(err)
      alert('Download & Delete failed')
    }
  }

  // Sync All (Download ALL + Delete ALL)
  const syncAll = async () => {
    if (files.value.length === 0) {
      alert('No files to sync')
      return
    }

    if (!confirm(`Download and delete all ${files.value.length} files?`)) {
      return
    }

    isSyncing.value = true

    let successCount = 0
    let failCount = 0

    // Loop through a COPY of the list (since we'll be removing items)
    const fileIds = files.value.map(f => f.id)

    for (const id of fileIds) {
      const success = await downloadAndDelete(id)
      if (success) {
        successCount++
        // Remove from local list immediately
        files.value = files.value.filter(f => f.id !== id)
      } else {
        failCount++
      }
    }

    isSyncing.value = false
    alert(`Sync complete: ${successCount} downloaded, ${failCount} failed`)
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
.container {
  width: 100vw;
  height: 100vh;
  display: flex;
  flex-direction: column;
  align-items: center;
}

.loginContainer {
  min-width: 400px;
  padding-top: 25vh;
}

.login {
  position: relative;
  padding: 20px;
  border-radius: 20px;
  background-color: var(--bgd);
  display:flex;
  flex-direction: row;
  gap: 15px;
  align-items: stretch;
  border: var(--border) solid 3px;
  justify-content: space-around;
  height: fit-content;
}

.loginMsg {
  color: var(--border);
  position: absolute;
  bottom:100%;
  left: 20px;
  padding: 3px;
  font-weight: 600; 
}

.loginLeft {
  display:flex;
  flex:1;
  flex-direction: column;
  gap: 10px;
}

.loginInput {
  width: 100%;
  padding:10px;
  border-radius:10px;
  outline: none;
  box-shadow: none;
  border: var(--bgd) solid 3px;
  transition: all 0.2s ease;
}

.loginInput:hover {
  border-color: var(--border);
}

.loginInput:focus {
  outline: var(--borderd) 2px solid;
  border-color: var(--bgd);
}


.loginRight {
  height: auto;
}
.loginBtn {
  position: relative;
  padding: 10px;
  width: 76px;
  border: var(--borderd) solid 3px;
  border-radius: 10px;
  background-color: var(--border);
  height: 50%;
  display: flex;
  align-content: flex-start;
  flex-direction: column;
  border-bottom-right-radius: 0px;
  transition: all 0.2s ease; 
  color: var(--text)
}


.loginBtn:hover, .loginBtn:hover::before {
  border-color: var(--border);
  background-color: var(--borderd);
}


.loginBtn:active, .loginBtn:active::before {
  border-color: var(--text);
  background-color: var(--accd);
  /* outline: var(--border) 2px solid; */
}

.loginBtn::after {
  content: '';
  position: absolute;
  right: calc(80%);
  top: calc(100% + 3px);
  width: calc(20% + 6px);
  height: calc(100% + 6px);
  background-color: var(--bgd);
  border: var(--bgd) solid 3px;
}

.loginBtn::before {
  content: '';
  position: absolute;
  left: calc(20%);
  top: calc(100%);
  width: calc(80% + 3px);
  height: calc(100% + 9px);
  background-color: var(--border);
  border: var(--borderd) solid 3px;
  border-bottom-left-radius: 10px;
  border-bottom-right-radius: 10px;
  border-top: none;
  transition: all 0.2s ease;
}

.appContent {

  display: flex;
  flex-direction: column;
  gap: 20px;
  padding-top: 15vh;
  width: 50vw;
}

.block {
  position: relative;
  padding: 20px;
  border-radius: 20px;
  background-color: var(--bgd);
  display:flex;
  flex-direction: column;
  gap: 15px;
  align-items: stretch;
  border: var(--border) solid 3px;
  justify-content: space-around;
  height: auto;
  color: var(--text);
  width: 100%;
}

.fileInput {
  width: 100%;
  aspect-ratio: 3;
  background-color: var(--border);
  
}

.form {
  /* background-color: brown; */
  height: fit-content;
  display: flex;
  flex-direction: column;
  gap: 10px;
  width: 100%;
}

.fieldContainer {
  /* min-height: 100px; */
  flex: 1;
  display:flex;
  flex-direction: row;
  justify-content: space-between;
  /* min-height: 40px; */
  min-height: 40px;
  align-items: stretch;
  /* background-color: brown; */
  padding: 0px 30px;
}

.uploadBtn {
  border: 2px solid var(--bgd);
  padding: 2px 20px;
  border-radius: 10px;
  background-color: var(--border);
  color: var(--text);
  font-weight: 600;
  transition: all 0.2s ease;
  display: flex;
  align-items: center;
}
.uploadBtn:hover {
  border: 2px solid var(--border);
  background-color: var(--borderd);
}
.uploadBtn:active {
  border: 2px solid var(--bgd);
  outline: 2px solid var(--border);
  background-color: var(--borderd);
}

.bottomLeft {
  display:flex;
  flex-direction: row;
  justify-content: space-between;
  align-items: center;
  gap: 10px;

  
}

.refreshBtn {
  aspect-ratio: 1;
  border: 2px solid var(--bgd);
  border-radius: 50vh;
  background-color: var(--border);
  color: var(--text);
  transition: all 0.2s ease;
  height: 40px;
  padding: 5px;
  outline: none;
  outline-color: var(--text);
  display: flex;
  align-items: center;
  justify-content: center;
}

.refreshSvg {
  width: 25px;
  height:25px;
  stroke: white;
}

.refreshBtn:hover {
  border: 2px solid var(--border);
  background-color: var(--borderd);
  color: var(--bgd)
}
.refreshBtn:active {
  border: 2px solid var(--bgd);
  outline: 2px solid var(--border);
  background-color: var(--borderd);
}

.fileHeader {
  display: flex;
  flex-direction: row;
  width: 100%;
  /* gap: 10px; */
  justify-content: center;
  flex: 1;
  position: relative;
  padding-bottom: 10px;
}

.fileHeader:after {
  content: '';
  position: absolute;
  width: 80%;
  height: 1px;
  top: 100%;
  left: 10%;
  background-color: var(--border);
}

.uploadSvg {
  width: 40px;
  height: 40px;
}

.wrapper {
  position: relative;
  display: flex;
  flex-direction: row;
}

.refreshBtn1 {
  position: absolute;
  left: 5px;
  top: 10px;
  z-index: 10000000;
}

.deleteAll {
  position: absolute;
  right: 10px;
  top: 10px;
  scale: 1.1;
  z-index: 1000000000000;
}

.downloadAll {
  position: absolute;
  left: calc(100% + 10px);
  top: calc(50% - 15px);
  background-color: var(--bgd) !important;
}


.deviceSelect {
  width: 100%;
  padding: 10px 35px 10px 10px;   
  border-radius: 10px;
  outline: none;
  box-shadow: none;
  border: var(--bgd) solid 3px;
  background-color: var(--bgd);    
  color: var(--text);
  font-weight: 600;               
  transition: all 0.2s ease;

  appearance: none;
  -webkit-appearance: none;
  -moz-appearance: none;

  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='16' height='16' viewBox='0 0 24 24' fill='none' stroke='%23ffffff' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpolyline points='6 9 12 15 18 9'%3E%3C/polyline%3E%3C/svg%3E");
  background-repeat: no-repeat;
  background-position: right 10px center;
  background-size: 18px;
}

.deviceSelect:hover {
  border-color: var(--border);
}

.deviceSelect:focus {
  outline: var(--borderd) 2px solid;
  border-color: var(--bgd);
}

.fileList {
  padding-top: 10px;
  list-style: none;
  padding-left: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.fileItem {
  position: relative;
  display: flex;
  flex-direction: row;
  justify-content: space-between;
  /* background-color: blue; */
  align-items: center;
}

.fileItem:after {
  position: absolute;
  top: 100%;
  left:25%;
  width: 50%;
  height: 1px;
  content: '';
  background-color: var(--borderd);
}

.fileItem:last-child::after {
  display:none
}


.fileInfo {
  display: flex;
  flex-direction: row;
  gap: 12px;
}

.date {
  font-weight: bold;
  cursor: default;
}

.sender {
  cursor: default;
}

.size {
  color: var(--border);
  font-weight: bold;
  cursor: default;
}

.delete {
  aspect-ratio: 1;
  border: none;
  outline-color: white;
  outline: none;
  background-color: var(--border);
  display: flex;
  align-items: center;
  justify-content: center;
  height:30px;
  border-radius: 5px;
  border: solid 2px var(--bgd);
  transition: all 0.2s ease;
}

.delete:hover {
  background-color: var(--borderd);
  border-color: var(--border);
  cursor: pointer;
}

.delete:active {
  border-color: var(--bgd);
  outline: 1px solid var(--border);
}

.fileLink {
  color: var(--border);
  transition: all 0.2s ease;
}

.fileLink:hover {
  transform: scale(1.1);
}
</style>