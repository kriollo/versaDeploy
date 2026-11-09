// chapter links under a video jump to that time: <a data-video="id" data-t="seconds">
document.addEventListener("click", (e) => {
  const a = e.target.closest("a[data-video][data-t]");
  if (!a) return;
  const v = document.getElementById(a.dataset.video);
  if (!v) return;
  e.preventDefault();
  v.currentTime = Number(a.dataset.t);
  v.play();
  v.scrollIntoView({ behavior: "smooth", block: "center" });
});
