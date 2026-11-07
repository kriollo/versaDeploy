<script setup>
// only the server render has process.version: the marker proves SSR
const ssr = useState('ssr', () => (import.meta.server ? `ssr ${process.version}` : 'client'));
</script>

<template>
    <p>versa-e2e nuxt {{ ssr }}</p>
</template>
