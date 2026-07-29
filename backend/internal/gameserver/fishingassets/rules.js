(function installFishingRules() {
  const button = document.querySelector("#rulesButton");
  const modal = document.querySelector("#rulesModal");
  const closeButton = document.querySelector("#rulesClose");
  if (!button || !modal || !closeButton) return;

  const close = () => {
    modal.hidden = true;
    button.focus();
  };

  button.addEventListener("click", () => {
    modal.hidden = false;
    closeButton.focus();
  });
  closeButton.addEventListener("click", close);
  modal.addEventListener("click", (event) => {
    if (event.target === modal) close();
  });
  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && !modal.hidden) close();
  });
})();
