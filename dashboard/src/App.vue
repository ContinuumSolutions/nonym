<template>
  <div id="app">
    <Transition name="fade" mode="out-in">
      <router-view />
    </Transition>
    <ToastContainer />
  </div>
</template>

<script>
import ToastContainer from './components/ToastContainer.vue'

export default {
  name: 'App',
  components: {
    ToastContainer
  }
}
</script>
