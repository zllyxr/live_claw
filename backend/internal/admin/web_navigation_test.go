package admin

import (
	"strings"
	"testing"
)

func TestAdminMultiModulePagesUseSecondaryNavigation(t *testing.T) {
	application, err := webFiles.ReadFile("web/static/app.js")
	if err != nil {
		t.Fatalf("read embedded admin application: %v", err)
	}
	source := string(application)

	sections := map[string][]string{
		"users":    {"用户列表", "团队管理"},
		"wallet":   {"资金流水", "后台调账", "充值订单", "提现订单"},
		"payments": {"支付通道", "收款银行卡", "充值商品", "充值订单"},
		"games":    {"游戏目录", "捕鱼场次"},
		"lottery":  {"彩种列表", "彩票分类", "期号管理"},
		"sports":   {"赛事管理", "同步状态"},
		"bets":     {"彩票投注", "体育投注", "游戏结算"},
		"rbac":     {"管理员", "角色", "权限字典"},
		"system":   {"系统设置", "审计日志"},
	}
	for route, labels := range sections {
		routeDefinition := route + ": ["
		if !strings.Contains(source, routeDefinition) {
			t.Errorf("route %q has no pageSections definition", route)
		}
		for _, label := range labels {
			if !strings.Contains(source, "\""+label+"\"") {
				t.Errorf("route %q is missing section %q", route, label)
			}
		}
	}

	for _, required := range []string{
		"function sectionNavigation(route, activeSection)",
		"role=\"tablist\"",
		"aria-selected=\"",
		"requestedPath[1]",
		"state.routeKey = route + (section ? \"/\" + section : \"\")",
	} {
		if !strings.Contains(source, required) {
			t.Errorf("secondary navigation contract is missing %q", required)
		}
	}
}

func TestAdminPaymentActionsExplainUnavailableConditions(t *testing.T) {
	application, err := webFiles.ReadFile("web/static/app.js")
	if err != nil {
		t.Fatalf("read embedded admin application: %v", err)
	}
	source := string(application)

	for _, required := range []string{
		"当前账号可编辑支付配置",
		"只读：缺少 payments.write 权限",
		"请先完成通道配置",
		"配置并通过协议检查后可启用",
		"请先新增并启用收款银行卡",
		"该服务商尚未接入服务端",
		"管理银行卡",
	} {
		if !strings.Contains(source, required) {
			t.Errorf("payment action state is missing explanation %q", required)
		}
	}
}

func TestSystemSettingsUseFriendlyFormsInsteadOfRawJSON(t *testing.T) {
	application, err := webFiles.ReadFile("web/static/app.js")
	if err != nil {
		t.Fatalf("read embedded admin application: %v", err)
	}
	source := string(application)

	for _, key := range []string{
		"platform.brand",
		"content.pages",
		"security.session",
		"invite.policy",
		"wallet.policy",
		"game.fishing",
		"live.provider",
		"im.policy",
		"lottery.policy",
		"app.update",
	} {
		if !strings.Contains(source, `"`+key+`": {`) {
			t.Errorf("system setting %q has no friendly form definition", key)
		}
	}

	for _, required := range []string{
		"function systemSettingCards(items)",
		"function systemSettingFormFields(row)",
		"function parseSystemSettingField(field, rawValue)",
		"按业务含义填写并保存，无需编写 JSON",
		"场次与倍率",
		"只允许一台设备登录",
		"新版本覆盖比例",
	} {
		if !strings.Contains(source, required) {
			t.Errorf("friendly system setting form is missing %q", required)
		}
	}

	for _, legacy := range []string{
		`{ label: "值", render: (row) => '<pre class="json-block">'`,
		`label: row.is_secret ? "新的 JSON 密钥值" : "JSON 值"`,
		`{ name: "is_secret", label: "类型"`,
	} {
		if strings.Contains(source, legacy) {
			t.Errorf("legacy raw JSON setting editor is still present: %q", legacy)
		}
	}
}
