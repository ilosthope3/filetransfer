










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
              <svg class="uploadSvg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                <path d="M12 4v12m0-12l-3 3m3-3l3 3M4 16v2a2 2 0 002 2h12a2 2 0 002-2v-2" stroke-linecap="round" stroke-linejoin="round"/>
              </svg>
            </button>
          </div>
          
          
        </form>
        

        
      </div>
    
      
      <div v-if="files.length != 1" class="block">
        <div class="fileHeader">
          <div class="wrapper">
            <h2>Your Files</h2>
            <button class="refreshBtn refreshBtn1" @click="fetchFiles">
              <svg fill="#ffffff" class="refreshSvg" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" version="1.1" id="Capa_1" x="0px" y="0px" viewBox="0 0 512 512" style="enable-background:new 0 0 512 512;" xml:space="preserve" width="512" height="512">                 
                <path d="M489.797,256c-10.791-0.141-19.924,7.939-21.099,18.667c-9.959,117.754-113.491,205.138-231.245,195.179   S32.315,356.354,42.275,238.6S155.766,33.462,273.52,43.421c50.983,4.312,98.733,26.75,134.592,63.245h-66.603   c-11.782,0-21.333,9.551-21.333,21.333s9.551,21.333,21.333,21.333h88.384c21.874-0.012,39.604-17.742,39.616-39.616V21.333   C469.509,9.551,459.958,0,448.176,0c-11.782,0-21.333,9.551-21.333,21.333v44.331C321.548-28.425,159.915-19.341,65.826,85.954   s-85.005,266.927,20.29,361.016s266.927,85.005,361.016-20.29c36.575-40.931,59.007-92.547,63.977-147.214   c1.096-11.814-7.593-22.279-19.407-23.375C491.069,256.033,490.434,256.002,489.797,256z"/>
              </svg>
            </button>
          </div>
        </div>
        <ul class="fileList">
          <li class="fileItem" v-for="file in files" :key="file.name">{{ file.filename }} - {{file.timestamp}}</li>
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
  const link = ref('')
  const receiver = ref('')
  const message = ref('')
  const deviceNames = ref([])
  const dropZoneRef = ref(null)
  const inputFiles = ref([])
  /////////////////////////////////////////////////////////////////////////////////

  const handleFileChange = async (files) => {
    inputFiles.value = files
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
        
        console.log(data, '111111111111111')
        files.value = data.data
      }
    } catch (err) {
      console.error('Failed to fetch files', err)
    }
  }

  const handleUpload = async (files) => {

    message.value = ""

    if (files.length === 0 && !link.value) {
      message.value = 'Select a file or paste a link'
      return
    }

    const formData = new FormData()
    for (let i = 0; i < files.length; i++) {
      formData.append('file', files[i])
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
        files = []
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
  padding: 0 20px;
  border-radius: 10px;
  background-color: var(--border);
  color: var(--text);
  font-weight: 600;
  transition: all 0.2s ease;
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
  left: calc(100% + 10px);
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

</style>