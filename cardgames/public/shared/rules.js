(function installGameRules() {
  const commonSettlement = [
    '每局使用平台钱包资金，入桌时统一收取100星币。',
    '中途退出或断网不会取消牌局，将转为离线托管并继续结算。',
    '本局结束后，奖励和输赢直接结算回原用户平台钱包。'
  ];
  const rules = {
    ddz: {
      title: '斗地主玩法说明',
      sections: [
        ['基本玩法', ['3人对战，使用54张牌；每人17张，剩余3张为地主底牌。', '依次选择叫地主或不叫，确定地主后地主获得3张底牌。']],
        ['牌型规则', ['支持单张、对子、三张、三带一、三带二、顺子、连对、飞机、四带二、炸弹和王炸。', '同牌型按主点数比较；炸弹可压普通牌型，王炸最大。']],
        ['回合规则', ['轮到自己时选择牌后出牌；无法或不想压牌时可选择“不要”。', '普通操作15秒；超时会自动不要，必须出牌时自动选择合法牌。']],
        ['倍数与胜负', ['地主先出完则地主胜；任一农民先出完则农民方胜。', '炸弹或王炸会提高倍数，平台钱包桌最高封顶32倍。']]
      ]
    },
    mahjong: {
      title: '麻将玩法说明',
      sections: [
        ['基本玩法', ['4人对战，使用万、条、筒、东南西北中发白共136张牌。', '手牌按万、条、筒、字牌自动排序；本玩法不开放吃牌。']],
        ['操作规则', ['可碰、明杠、暗杠；碰或杠后按提示继续打出一张牌。', '可组成4组面子加1对将，或七小对时可以胡牌。']],
        ['番型说明', ['支持平胡、碰碰胡、清一色、七小对和杠加番等结算番型。', '自摸由其余三家共同支付；点炮由出牌方承担。']],
        ['超时规则', ['普通出牌15秒；超时优先打出刚摸到的牌。', '碰、杠、胡选择限时8秒；超时自动选择“过”。']]
      ]
    },
    mahjong_red: {
      title: '红中麻将玩法说明',
      sections: [
        ['基本玩法', ['4人对战，使用136张麻将牌，手牌按万、条、筒、字牌排序。', '红中作为赖子，可辅助组成顺子、刻子或将牌；红中不能用于碰杠。']],
        ['操作规则', ['不开放吃牌；可碰、明杠、暗杠，并支持自摸或点炮胡。', '胡牌支持标准4面子1将和七小对。']],
        ['番型说明', ['支持平胡、碰碰胡、清一色、七小对、杠加番及红中规则标识。', '结算由服务器统一判定，客户端不能修改牌型结果。']],
        ['超时规则', ['普通出牌15秒；超时优先打出刚摸到的牌。', '碰、杠、胡选择限时8秒；超时自动选择“过”。']]
      ]
    },
    paodekuai: {
      title: '跑得快玩法说明',
      sections: [
        ['基本玩法', ['3人对战，使用去掉2和大小王后的48张牌，每人16张。', '持有黑桃3的玩家先手，第一手必须包含黑桃3。']],
        ['牌型规则', ['支持单张、对子、三张、顺子、连对、飞机、三带和炸弹等合法组合。', '跟牌必须使用相同牌型并压过上家；两家都不要后重新自由出牌。']],
        ['胜负规则', ['最先打完全部手牌的玩家获胜。', '手牌按点数从大到小自动排序，提示只会给出服务器判定的合法出法。']],
        ['超时规则', ['普通操作15秒；能不要时自动不要。', '必须出牌或首手时，超时自动选择一组合法牌。']]
      ]
    },
    zhajinhua: {
      title: '炸金花玩法说明',
      sections: [
        ['基本玩法', ['3人对战，每人3张牌；入桌资金转为本桌筹码，每局先下基础注。', '可选择看牌、过牌、跟注、加注、比牌或弃牌。']],
        ['牌型大小', ['豹子 ＞ 同花顺 ＞ 金花 ＞ 顺子 ＞ 对子 ＞ 单张。', '同牌型依次比较主要点数；A可作为A23顺子的低位牌。']],
        ['开牌规则', ['其他玩家全部弃牌时，剩余玩家直接获胜。', '最多进行10轮下注；达到上限后自动开牌比较。']],
        ['超时规则', ['普通操作15秒；无需跟注时自动过牌。', '需要继续投入筹码时，超时自动弃牌，不替用户追加下注。']]
      ]
    }
  };

  const path = location.pathname.toLowerCase();
  let key = Object.keys(rules).find((name) => path.includes(`/${name}/`));
  if (path.includes('/mahjong/')) {
    key = new URLSearchParams(location.search).get('variant') === 'red' ? 'mahjong_red' : 'mahjong';
  }
  const config = rules[key];
  if (!config) return;

  document.body.classList.add('rules-enabled');
  const button = document.createElement('button');
  button.className = 'game-rules-button';
  button.type = 'button';
  button.textContent = '规则';
  button.setAttribute('aria-label', `打开${config.title}`);

  const modal = document.createElement('section');
  modal.className = 'game-rules-modal';
  modal.hidden = true;
  modal.setAttribute('role', 'dialog');
  modal.setAttribute('aria-modal', 'true');
  modal.setAttribute('aria-label', config.title);
  const sections = config.sections.concat([['资金与退出', commonSettlement]]);
  modal.innerHTML = `
    <article class="game-rules-card">
      <header class="game-rules-head">
        <div><small>GAME GUIDE</small><strong>${config.title}</strong></div>
        <button class="game-rules-close" type="button" aria-label="关闭规则">×</button>
      </header>
      <div class="game-rules-content">
        ${sections.map(([title, items], index) => `
          <section class="game-rule-section${index === sections.length - 1 ? ' wallet' : ''}">
            <h3>${title}</h3>
            <ul>${items.map((item) => `<li>${item}</li>`).join('')}</ul>
          </section>`).join('')}
      </div>
    </article>`;

  const close = () => {
    modal.hidden = true;
    button.focus();
  };
  button.addEventListener('click', () => {
    modal.hidden = false;
    modal.querySelector('.game-rules-close').focus();
  });
  modal.querySelector('.game-rules-close').addEventListener('click', close);
  modal.addEventListener('click', (event) => {
    if (event.target === modal) close();
  });
  document.addEventListener('keydown', (event) => {
    if (event.key === 'Escape' && !modal.hidden) close();
  });
  document.body.append(button, modal);
})();
