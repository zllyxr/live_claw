const filters = [...document.querySelectorAll(".filter")];
const cards = [...document.querySelectorAll(".game-card")];
const count = document.querySelector("#result-count");
const emptyState = document.querySelector(".empty-state");

function showGames(player) {
  let visible = 0;

  cards.forEach((card) => {
    const matches = player === "all" || card.dataset.players.split(" ").includes(player);
    card.hidden = !matches;
    if (matches) visible += 1;
  });

  count.textContent = `共 ${visible} 款`;
  emptyState.hidden = visible !== 0;
}

filters.forEach((filter) => {
  filter.addEventListener("click", () => {
    filters.forEach((item) => {
      const active = item === filter;
      item.classList.toggle("is-active", active);
      item.setAttribute("aria-pressed", String(active));
    });
    showGames(filter.dataset.player);
  });
});

const copyButton = document.querySelector("#copy-command");

copyButton.addEventListener("click", async () => {
  try {
    await navigator.clipboard.writeText(copyButton.dataset.copy);
    copyButton.textContent = "已复制";
    window.setTimeout(() => {
      copyButton.textContent = "复制";
    }, 1600);
  } catch {
    copyButton.textContent = "请手动复制";
  }
});
