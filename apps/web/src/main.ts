import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import './styles/main.css'
import 'highlight.js/styles/github-dark.css'
import '@fontsource/roboto/latin-400.css'
import '@fontsource/roboto/latin-500.css'
import '@fontsource/roboto/latin-700.css'

createApp(App).use(router).mount('#app')
