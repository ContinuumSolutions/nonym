<template>
  <div id="app">
    <Transition name="fade" mode="out-in">
      <router-view />
    </Transition>
  </div>
</template>

<script>
export default {
  name: 'App'
}
</script>
