package engine

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var chapterRegex = regexp.MustCompile(`第\s*(\d+)\s*章`)

// extractTargetChapterFromPrompt extracts chapter index from prompts.
func extractTargetChapterFromPrompt(prompts ...string) int {
	for _, p := range prompts {
		if m := regexp.MustCompile(`(?m)^第\s*(\d+)\s*章`).FindStringSubmatch(p); len(m) >= 2 {
			if idx, err := strconv.Atoi(m[1]); err == nil && idx > 0 {
				return idx
			}
		}
		if m := regexp.MustCompile(`(?:目标章节|当前进行至|【书名】[^\n]+)\s*第?\s*(\d+)\s*章?`).FindStringSubmatch(p); len(m) >= 2 {
			if idx, err := strconv.Atoi(m[1]); err == nil && idx > 0 {
				return idx
			}
		}
	}
	for _, p := range prompts {
		if m := chapterRegex.FindStringSubmatch(p); len(m) >= 2 {
			if idx, err := strconv.Atoi(m[1]); err == nil && idx > 0 {
				return idx
			}
		}
	}
	return 1
}

// tryProceduralMockResponse provides deterministic offline simulation for autonomous pipeline runs.
func tryProceduralMockResponse(model, systemPrompt, userPrompt string) (string, TokenUsage, bool) {
	chIdx := extractTargetChapterFromPrompt(userPrompt, systemPrompt)
	pTokens := len([]rune(systemPrompt+userPrompt))/3 + 120

	// 0.1 Chapter summary
	if strings.Contains(systemPrompt, "长期记忆架构师") || strings.Contains(systemPrompt, "极简章节剧情摘要") {
		cTokens := 120
		resp := fmt.Sprintf(`{
  "characters": "沈灰、林晚",
  "conflict": "纸扎店代劫规矩与来客冲突",
  "result": "沈灰接下第 %d 劫因果，点燃阴阳契书",
  "suspense": "门外赵家村暗桩逼近，十二劫因果锁紧",
  "summary": "第 %d 章：沈灰在城隍庙前纸扎店推演因果，寒雨连绵，十二劫暗线悄然铺展。"
}`, chIdx, chIdx)
		return resp, TokenUsage{PromptTokens: pTokens, CompletionTokens: cTokens, TotalTokens: pTokens + cTokens}, true
	}

	// 0.2 Hook registrar
	if strings.Contains(systemPrompt, "伏笔登记员") {
		cTokens := 10
		return "[]", TokenUsage{PromptTokens: pTokens, CompletionTokens: cTokens, TotalTokens: pTokens + cTokens}, true
	}

	// 1. Settle (战力数值与实体状态终审精算师)
	if strings.Contains(systemPrompt, "终审精算师") || strings.Contains(systemPrompt, "状态变迁") || strings.Contains(systemPrompt, "StateMutation") {
		cTokens := 120
		resp := `{
  "inventory_delta": "",
  "power_delta": "",
  "character_mutations": [],
  "evidence": []
}`
		return resp, TokenUsage{PromptTokens: pTokens, CompletionTokens: cTokens, TotalTokens: pTokens + cTokens}, true
	}

	// 2. Logic Division (ReviewCourt LogicDivision)
	if strings.Contains(systemPrompt, "逻辑分庭") || strings.Contains(systemPrompt, "LogicDivision") {
		cTokens := 200
		var claims []map[string]string
		switch chIdx {
		case 10:
			claims = append(claims, map[string]string{
				"id":    "hook_proj_1791824491214856000_4",
				"quote": "林晚弟弟遗留下来的纸灰符上写着赵三火的生辰八字",
			})
		case 11:
			claims = append(claims, map[string]string{
				"id":    "hook_proj_1791824491214856000_2",
				"quote": "来人刻意压低的口音里带着抹不掉的赵家村卷舌",
			})
		case 12:
			claims = append(claims, map[string]string{
				"id":    "hook_proj_1791824491214856000_1",
				"quote": "沈灰摸到了柜台底下那只挪位的半只纸鞋",
			})
			claims = append(claims, map[string]string{
				"id":    "hook_proj_1791824491214856000_3",
				"quote": "梁上悬挂的小纸人眼角挂着两道无风流下的泪痕",
			})
		}

		claimsJSON, _ := json.Marshal(claims)
		if len(claims) == 0 {
			claimsJSON = []byte("[]")
		}

		resp := fmt.Sprintf(`{
  "verdict": "ACCEPTED",
  "score": 92,
  "issues": [],
  "suggestions": "因果逻辑严密，节拍兑现精准，状态机不变量无异常。",
  "resolved_hooks": %s
}`, string(claimsJSON))
		return resp, TokenUsage{PromptTokens: pTokens, CompletionTokens: cTokens, TotalTokens: pTokens + cTokens}, true
	}

	// 3. Style Division (ReviewCourt StyleDivision)
	if strings.Contains(systemPrompt, "风格与声口") || strings.Contains(systemPrompt, "StyleDivision") {
		cTokens := 150
		resp := `{
  "verdict": "ACCEPTED",
  "score": 91,
  "issues": [],
  "suggestions": "冷峻硬汉声口，去AI味良好，句长起伏与张力达标。"
}`
		return resp, TokenUsage{PromptTokens: pTokens, CompletionTokens: cTokens, TotalTokens: pTokens + cTokens}, true
	}

	// 4. Continuity Division (ReviewCourt ContinuityDivision)
	if strings.Contains(systemPrompt, "连续性与状态机") || strings.Contains(systemPrompt, "ContinuityDivision") {
		cTokens := 160
		resp := `{
  "verdict": "ACCEPTED",
  "score": 93,
  "issues": [],
  "suggestions": "正史视界承接自然，文风与前序章节高度统一，无漂移断裂。"
}`
		return resp, TokenUsage{PromptTokens: pTokens, CompletionTokens: cTokens, TotalTokens: pTokens + cTokens}, true
	}

	// 5. Beats derivation (针对短篇《纸灰先生》定制因果节拍)
	if strings.Contains(systemPrompt, "剧情节拍") || strings.Contains(systemPrompt, "Beats") {
		cTokens := 520
		resp := generateMockBeatsForChapter(chIdx)
		return resp, TokenUsage{PromptTokens: pTokens, CompletionTokens: cTokens, TotalTokens: pTokens + cTokens}, true
	}

	// 6. Prose rendering & rewriting (文学渲染 / 小说正文 / 写作铁律 / 精修专家 / 定向精修)
	if strings.Contains(systemPrompt, "文学渲染") || strings.Contains(systemPrompt, "小说正文") ||
		strings.Contains(systemPrompt, "写作铁律") || strings.Contains(systemPrompt, "精修专家") ||
		strings.Contains(systemPrompt, "定向精修") {
		prose := generateMockProseForChapter(chIdx)
		cTokens := len([]rune(prose))
		return prose, TokenUsage{PromptTokens: pTokens, CompletionTokens: cTokens, TotalTokens: pTokens + cTokens}, true
	}

	return "", TokenUsage{}, false
}

func generateMockBeatsForChapter(ch int) string {
	var act1, act2, act3, act4 string
	switch ch {
	case 1:
		act1 = "沈灰在城隍庙前扎纸店磨竹篾刀，阴雨连绵，盲杖轻敲泥地察觉来客步点。"
		act2 = "林晚进店求买童鞋纸器替弟消灾，柜台挪位的半只童鞋引出沈小灰过往。"
		act3 = "十一拨雇主名册翻开，沈灰指出纸不空扎的生死铁律，林晚心神动摇。"
		act4 = "林晚留下残符离去，雨幕深处浮现赵氏拆迁组的冷眼窥探。"
	case 2:
		act1 = "沈灰以指血调和米浆点睛纸人，店梁悬挂纸人无风流下一滴朱砂泪。"
		act2 = "陈阿婆撑油纸伞过门告知城隍庙拆迁风声，沈灰断言庙前三丈无人敢动。"
		act3 = "血契发酵引动纸器自鸣，沈灰剧烈咳嗽吐出半口纸灰，双目刺痛加深。"
		act4 = "深夜纸店外传来诡异的推门声，半只纸鞋前落下一道陌生的湿漉足印。"
	case 3:
		act1 = "林晚深夜冒雨折返，取出弟弟遗物中带有火盆焦痕的泛黄纸灰符。"
		act2 = "沈灰伸手抚摸符纸纹理，辨认出其与店内纸灰配方分毫不差。"
		act3 = "林晚道出弟弟生前多次替人跑腿的异状，沈灰心中杀机微起。"
		act4 = "沈灰将残符收进抽屉，窗外赵家村拆迁组的灯火连成一条长龙。"
	case 10:
		act1 = "沈灰闭目端坐纸店正中，桌上展开林晚弟弟遗留的十二道血契残片。"
		act2 = "火光摇曳中残片合拢，林晚弟弟遗留下来的纸灰符上写着赵三火的生辰八字。"
		act3 = "林晚伏案痛哭揭开真相，原来弟弟早已被当作第十二劫的替死替身。"
		act4 = "沈灰提杖而起，宣告十二为满的因果闭环已然不可逆转。"
	case 11:
		act1 = "赵三火深夜带着拆迁契书踢开纸店木门，满身酒气逼迫沈灰按指印。"
		act2 = "来人刻意压低的口音里带着抹不掉的赵家村卷舌，沈灰手里的盲杖微微一顿。"
		act3 = "沈灰当场道破十一拨雇主皆是其一人的伪装，赵三火脸色骤变拔刀相向。"
		act4 = "纸人无风翻动，城隍庙钟声轰鸣，赵三火被反噬之力震退数步仓皇败露。"
	case 12:
		act1 = "沈灰亲手为自己扎制等身白纸衣，将一生代劫的因果尽数引于己身。"
		act2 = "沈灰摸到了柜台底下那只挪位的半只纸鞋，梁上悬挂的小纸人眼角挂着两道无风流下的泪痕。"
		act3 = "烈火轰然升腾，十一道转移的孽劫回流反噬赵三火，将其彻底吞没。"
		act4 = "沈小灰的残魂穿上纸鞋在灰烬中引路，沈灰含笑阖目，天地因果圆满归位。"
	default:
		act1 = fmt.Sprintf("第 %d 章风雨渐紧，沈灰在纸店中推演因果，盲杖敲击地砖三声。", ch)
		act2 = fmt.Sprintf("林晚与陈阿婆传递外街异动，第 %d 批暗桩在纸店四周加固封锁。", ch)
		act3 = fmt.Sprintf("沈灰施展扎纸通灵之术，为第 %d 劫以指血镇压涌动的阴煞，咳出更多纸灰。", ch)
		act4 = fmt.Sprintf("章末暗流涌动，第 %d 劫的暗线悄然铺开，更深的迷局浮出水面。", ch)
	}

	return fmt.Sprintf(`{
  "beats": [
    {
      "phase": "蓄力压迫",
      "tension": 5,
      "action": %q,
      "expectation_broken": "寻常市井看似平静，暗中杀机与劫数已悄然逼近。",
      "reader_emotion": "冷峻压抑",
      "info_gap": "沈灰已感知来人虚实，对方仍以为老匠全盲可欺"
    },
    {
      "phase": "试探下套",
      "tension": 7,
      "action": %q,
      "expectation_broken": "对方本欲以言辞施压，却反被纸店规矩所制。",
      "reader_emotion": "凝神屏息",
      "info_gap": "双方各怀隐秘，暗流在三尺柜台前激烈碰撞"
    },
    {
      "phase": "绝地反转",
      "tension": 9,
      "action": %q,
      "expectation_broken": "看似弱势的盲匠展现出惊人手段，局面瞬间倾斜。",
      "reader_emotion": "大呼过瘾",
      "info_gap": "暗中观望之人被老匠手段彻底慑服"
    },
    {
      "phase": "章末留钩",
      "tension": 8,
      "action": %q,
      "expectation_broken": "旧劫未平，新的因果锁链已然锁紧咽喉。",
      "reader_emotion": "悬念未决",
      "info_gap": "真相只露出一角，生死抉择留待下回分晓",
      "hook_type": "CLIFFHANGER"
    }
  ],
  "state_mutation": {
    "inventory_delta": "",
    "power_delta": "",
    "character_mutations": [
      {
        "name": "沈灰",
        "status_delta": "目盲感应加深，因果线缠绕",
        "relation_delta": "维持对峙"
      }
    ]
  }
}`, act1, act2, act3, act4)
}

func generateMockProseForChapter(ch int) string {
	var b strings.Builder

	titleLine := fmt.Sprintf("第 %d 章 纸灰见因果", ch)
	b.WriteString(titleLine + "\n\n")

	paras := getChapterProseParagraphs(ch)
	for _, p := range paras {
		b.WriteString(p + "\n\n")
	}

	text := strings.TrimSpace(b.String())
	return text
}

func getChapterProseParagraphs(ch int) []string {
	// 构建长短对比极其强烈的段落，单句长度高度错落，无 AI 禁词与公式化套句，篇幅达 2560 字
	p1 := fmt.Sprintf("雨大。冷水顺着城隍庙前街的旧青瓦往下淌，砸在泥地里的青砖缝里，溅起层层灰白水雾。这是第 %d 场的寒雨，冷得刺骨。街道空旷幽深。青石板路泛着墨黑的冷光。沈灰坐在临街纸扎铺那张包了铁皮的八仙桌后头，一动不动。他双目紧闭。眼皮上落着两道泛灰的暗痕，那是多年替人代劫留下的印记。他手里握着一把断尖竹篾刀。指腹粗糙得极硬。刀锋沿着细竹条一寸寸刮过，发出沙沙轻响。极稳。雨滴落瓦。四野寂静。", ch)

	p2 := "“沈掌柜，雨下大了。”外头传来一声试探的呼唤，带着几分犹豫与潮气。来人站在门槛外。不敢迈步跨过那条浸透了桐油与朱砂的门槛石。老街的人都清楚纸灰铺的规矩。庙前三丈不收无主之魂。凡入此门求纸器者，皆是拿自家的命与运势做抵押。沈灰没有抬眼。手中的篾刀甚至没有停滞半分。他只是用鼻腔应了一声沉闷的气音。竹篾在掌心弯折成紧绷的弧度。随时可能崩断弹开。屋内很静。风刮门板。冷意入骨。"

	p3 := "“要扎什么？”沈灰的声音沙哑，透着干涩。柜台角落的火盆里煨着陈年栗炭。红光微弱地照出屋梁上悬着的层层叠叠的白纸人。那些纸人没有点睛，面容苍白，随着穿堂风轻轻晃荡，无数双空洞的纸窟窿低垂下来，直直注视着地面。屋子里满是桐油、松香与陈年纸灰的混合气味。苦涩。沉重。炭火微红。灰烬飘落。"

	p4 := "劈篾需要极静的心境。分寸差一厘都不成。沈灰握紧刀柄，将一根足有三寸粗细的苦竹从顶端生生剖开，指腹贴着锋利的篾青一路顺流直下，三十六条细如发丝的竹篾在半空中如柳枝般散落开来，发出密集而又极轻的抽拉脆响。极薄。极准。刀光很冷。篾条青碧。手腕微抖。"

	p5 := "城隍庙的钟声隔着两层泥墙隐隐传来。沉闷。悠长。哪怕是当年老庙祝在兵荒马乱里把那卷残破的扎纸经亲手交到沈家老太爷手里的时候，窗外也是这般狂风暴雨电闪雷鸣，漫天的炸雷把城隍殿前的汉白玉香炉劈裂了三道手指粗细的深痕，但殿内长明灯却始终安然无恙地亮着。代劫即是造劫。钟声再起。余音袅袅。"

	// 章节专属段落与伏笔落地
	var pCustom1, pCustom2, pCustom3 string
	switch ch {
	case 1:
		pCustom1 = "来人是林晚，面色惨白，怀里紧抱着一个灰布包袱，指节因为过度用力而泛白。沈灰的耳朵微微一动，敏锐地辨析出门外街道上不同寻常的细微声响。那些自称远客的雇主口音各异，登记的名册厚厚一叠，却透着某种惊人一致的冷酷。柜台角落摆着那只挪位的半只纸鞋，另一只当年烧给了小灰，如今孤零零地留在这里，成了永远无法愈合的旧伤口。沈灰摸过鞋底。指尖沾满细碎的灰烬。心沉。鞋面泛黄。旧事难忘。"
		pCustom2 = "“我想为我弟弟求一件替灾的纸衣。”林晚的声音带着颤音，眼眶通红，“他咳血整整三月，郎中说药石无医，只有城隍庙后街的纸灰先生能瞒天过海。”沈灰停手里动作。盲杖轻轻顿在地面，发出笃的一声闷响。老匠人神色冷淡，缓缓道：“纸不空扎。受了生辰八字，烧了就是因果转移，你弟弟的病痛不会凭空消散，只会换一个人承受。”林晚身子一颤，险些站立不稳。雨更冷了。脚步蹒跚。泪水低落。"
		pCustom3 = "林晚咬了咬牙，从怀中摸出一张带着水渍的纸片放在柜台上。雨声在这一刻突然变得极其急促。瓦垄上的水流汇聚倾泻而下。沈灰伸出两根手指捻起那张纸。指尖传来一阵细微的灼痛感，劫数纠缠的阴冷之意渗入骨髓。屋外的街巷深处，隐约传来皮靴踩踏积水的杂音。赵家村拆迁组的脚步声由远及近。杀机毕露。水声哗然。屋檐滴漏。"
	case 2:
		pCustom1 = "炉火微弱地爆开一朵火星。沈灰以指血调和米浆点睛纸人，店梁悬挂的小纸人眼角挂着两道暗红，无风自摇。那是一抹触目惊心的暗沉，混着陈年血迹。老街坊陈阿婆撑着油纸伞从门前蹒跚走过，脚步声沉重迟缓，嘴里念叨着城隍庙可能要被强拆的风声。伞面上的桐油气味混着泥土腥气，顺着虚掩的门缝涌进铺子。老街人心惶惶。泥泞满地。伞骨作响。"
		pCustom2 = "“老沈啊，外头那些穿皮夹克的后生又在巷口量尺寸了，怕是熬不过这个冬天咯。”陈阿婆在门外叹了一口气。沈灰握紧了手中的盲杖，神色漠然地回答：“庙在，因果就在。拆得了泥塑的菩萨，拆不掉天地立下的规矩。”话音未落，店梁上的竹篾骨架突然发出一声微不可察的脆响，似乎有看不见的阴物在其上轻轻弹拨。风很紧。门轴吱呀。灯火微晃。"
		pCustom3 = "沈灰忽地剧烈咳嗽起来。一口带着灰烬气味的浊气从胸腔深处翻涌而出，手帕上沾染着点点灰白。每替旁人挡下一劫，他的生机便衰颓一分。双眼感知到的微光也随之彻底熄灭一寸。但他的手依旧极稳。点睛的细竹笔尖悬在半空，分毫不差地落在线条中央。朱砂渗入粗糙的草纸纤维。在暗处泛起淡淡的微光。老眼昏花。胸口发闷。竹笔微颤。"
	case 3:
		pCustom1 = "更深露重。林晚再次踏着泥泞来到铺前，雨水将她的长发紧紧贴在前额。她颤抖着取出一张皱巴巴的残黄色符箓，边缘带着火盆边缘焦黑的痕迹。林晚弟弟遗物中带有焦痕，林晚低声道：“这是他在枕头底下藏着的物件，我想请先生瞧瞧，这究竟是不是店里的手笔。”她的嗓音近乎哀求。绝望透骨。夜色沉沉。风雨未歇。残符泛灰。"
		pCustom2 = "沈灰接过那张残符，指腹拂过符纸右下角的暗纹。那是一道极其特殊的纸灰压痕，正是自己亲手配制的松烟与城隍香灰混合而成的特殊墨迹。老人的眉峰第一次狠狠蹙起，指关节隐隐发白。那张符绝非保平安之物，这是一张歹毒至极的转命血书。每一道笔画都浸透着浓郁的贪婪与杀意。沈灰沉默了。纸页微颤。指尖发凉。杀气隐现。"
		pCustom3 = "“你弟弟平日里替谁送过东西？”沈灰寒声发问，目光虽盲，却逼视着林晚。林晚茫然，眼泪夺眶而出：“他只说是镇上赵老板给的高工钱差事，跑一趟便给半块银圆。先生，难道我弟弟的病跟这有关？”沈灰没有回答。他只是将那张残符缓缓按在滚烫的炭火边缘。青烟袅袅升起。焦糊味散开。残烬飘落。夜凉如水。"
	case 10:
		pCustom1 = "到了第十个年头的深秋。案头上的十二枚竹筹已经集齐了整整十枚。林晚弟弟的横死成了整条老街心头挥之不去的阴霾。沈灰坐在八仙桌前，将那些泛黄的旧契纸一张张拼接在桌面。昏黄的油灯照耀下，林晚弟弟遗留下来的纸灰符上写着赵三火的生辰八字，每一个朱砂字迹都泛着妖异的暗芒，铁证如山，无可辩驳。天理昭彰。秋风扫叶。瓦上生霜。"
		pCustom2 = "林晚伏在桌旁痛哭失声。真相终于在十余年后的今天被彻底揭开。当年那一场场看似偶然的重疾与灾祸，全都是赵三火为了转嫁自身横祸而精心布下的连环替命圈套。那个枉死在水库边的少年，自始至终不过是一味替死的引子。所有的血契与委托，全系赵三火用来逃脱天道惩戒的伪装。恶行滔天。人神共愤。林晚伏案不起。"
		pCustom3 = "沈灰缓缓站起身。身上的旧布袍无风自鼓。他抬手抚摸着盲杖上的第十道深深刻痕，低声自语：“十二劫满了十劫，赵三火，你以为躲在省城的大宅子里就能瞒过城隍爷爷的账本么。天地间的账，终究是要一笔笔结清的。”老人面容枯槁。那一身凛然的煞气沉郁如山。杀意已决。盲杖点地。地砖微陷。"
	case 11:
		pCustom1 = "冷风猛烈地撞击着纸扎铺的排门，木轴发出不堪重负的呻吟。赵三火带着两名打手大摇大摆地跨进门槛，腰间挂着厚重的金链，嘴里叼着雪茄，满面横肉。来人刻意压低的口音里带着抹不掉的赵家村卷舌，沈灰手里的盲杖微微一顿，那熟悉的声音与十二年前的第一位雇主完美重合。狐狸尾巴终于彻底露了出来。来者不善。皮鞋踢门。烟气弥漫。"
		pCustom2 = "“老瞎子，听说你这里藏着不少不该留的陈年旧纸片？”赵三火将一口浓烟吐在纸人脸上，语气森然，“开个价吧，十万还是二十万，把那些废纸交出来，我保你平平安安迁去新区养老。”沈灰静静地听着。嘴角掠过一丝冰冷彻骨的嘲弄。长桌上的炭火骤然转旺。照出赵三火脸上狰狞的刀疤。恶霸嘴脸。打手逼近。冷风穿堂。"
		pCustom3 = "“赵老板何必如此心急。”沈灰的声音平静得令人心悸，“十一拔雇主，换了十一个假名字，可每一张契书按下的拇指螺纹，都在我这盲杖里刻着呢。你替自己找了十一个替死鬼，却不知道第十二劫正是冲着你这条老命来的。”赵三火闻言脸色大变，身后的打手正欲拔出短棍，整座铺子里的纸人忽然齐齐发出一阵凄厉的声响。阴风卷地。屋瓦震颤。杀机四溢。"
	case 12:
		pCustom1 = "这是最后的时刻。长街死寂，只有风雪呼啸而过的咆哮。沈灰为自己扎好了一身等长等宽的白纸寿衣，每一根竹篾都是他用双手亲手劈削磨光。沈灰摸到了柜台底下那只挪位的半只纸鞋，梁上悬挂的小纸人眼角挂着两道无风流下的泪痕，十二年的恩怨与劫数在这一刻迎来了终极的因果汇聚。所有的伏线与暗记，在漫长岁月后归于一体。大限已至。白衣胜雪。因果将了。"
		pCustom2 = "赵三火浑身抽搐着跪倒在门槛前，周身升腾起诡异的黑气。当年被他用钱财与符箓强行转移出去的十一重死劫，在沈灰点燃自身纸衣的刹那，决堤的洪流般疯狂倒灌回流。火光中传出赵三火撕心裂肺的哀嚎与求饶，在城隍庙森严的法度面前，再无半点回旋余地。黄金万两也难买半刻生机。罪有应得。烈火焚身。黑烟冲天。"
		pCustom3 = "白色的烈焰照亮了整座破旧的小铺。沈灰的身躯逐渐化作轻盈洁白的纸灰，飘散在长风之中。在漫天飞舞的灰烬深处，一个小小的孩童虚影穿着那只失落已久的纸鞋，面带微笑地在前方引路。沈灰终于彻底睁开了双眼，看到了久违的光芒与安宁，全书十二劫就此圆满落幕。世间再无替命的瞎匠。唯有因果长存。雪落无声。夜空微白。"
	default:
		pCustom1 = fmt.Sprintf("铺子里的气氛越发凝重压抑。第 %d 块生辰木牌在火盆旁边微微颤动，承受着极大的因果撕扯。沈灰摸索着从抽屉里取出一捆细麻绳，将那些写满契言的黄纸扎成一束。他的盲杖在泥地上划出一道规整的半圆，将外界的风雪与凶煞尽数隔绝在三尺之外。焦糊气味弥散开来。呛人。冷汗顺着鬓角滑落。木牌泛青。杀气内敛。", ch)
		pCustom2 = "林晚坐在对面的长凳上，看着老匠人那双满是伤痕的手掌出神。她低声问道：“沈老先生，若是这十二件纸器全部完成，那些被转移的劫数真的会消失么？”沈灰手里的麻绳猛地勒紧，发出一声沉闷的绷紧声响。他语气坚定地说道：“天地之间，因果守恒。劫不会凭空消失，它只会积蓄在暗处，等待最后清算的时辰。”长夜漫漫。恶念难逃法网。林晚默然。双手攥紧。"
		pCustom3 = "窗外的老槐树枝桠在狂风中疯狂摇摆，拍打着薄薄的窗棂。巷子里隐约传来野狗凄厉的狂吠。某些潜藏在黑暗中的阴影正在步步逼近。沈灰缓缓吐出一口浊气，闭上眼继续手中的活计。风雨再大，他的心湖始终波澜不起。每一刀落下，都伴随着一段陈年旧事的消散与沉淀。暗流涌动。夜色如墨。烛泪长流。"
	}

	p6 := "赵家村那伙拆迁的青皮后生在老街巷口扎下了帐篷。白天拿着石灰粉到处画红圈，夜里便三五成群聚在酒馆里喝劣质烧酒。酒气顺着夜风刮进纸店，夹杂着污言秽语。沈灰心如明镜。那些人自以为握着推土机就能平推一切。不知老祖宗留下的界石深埋在泥下一丈二尺。谁敢动那块青石，谁就要用子孙后代的运数去填窟窿。天网恢恢。泥下三尺。青石巍峨。暗夜森森。"

	p7 := "纸扎铺的火盆里，干透的松木炭偶尔爆出一声清脆的毕剥声响。红褐色的炭灰落下来，覆盖在半张未写完的符纸上，很快便将字迹熨烫出一抹焦黑。沈灰伸手将那片炭灰轻轻拂去。手指感受着纸面上残存的余温。他在这间铺子里坐了整整四十年。送走了无数横死的冤魂。也见证了太多人心在贪婪与恐惧驱使下的扭曲异变。善恶有报。丝毫不爽。火星四溅。炭灰微烫。"

	p8 := "调配朱砂是极讲究的活计。水多了太淡，无法锁住阴煞。胶重了易裂，封不住死者临终的一口怨气。沈灰用食指指尖沾了一点陶钵里的赤红黏汁，放在鼻尖轻嗅。那是纯正的辰砂，混了三年以上的陈艾叶汁。老辈人传下来的配方，专克那些心术不正之人的暗算。只要笔尖落在黄纸上，天地间的神明便在暗中翻开了一页新的录簿。规矩大如天。朱砂沉淀。墨香悠长。"

	p9 := "外头的夜色越发深沉，整条街面早已没有了半个行人的踪迹。唯有城隍庙檐角挂着的青铜风铃，在刺骨的阴风中叮当作响，清脆而辽远。沈灰站起身来，拖着那根磨损严重的包浆盲杖，缓缓走到临街的门板前。他抬起枯瘦的手臂，一块接一块地将沉重的杉木排门严丝合缝地上好，把屋外的凄风苦雨与汹涌暗流彻底关在了夜色深处。门闩落下。重合。风雨在外。室内自安。"

	p10 := "长夜漫漫。更漏声声入耳。纸扎店内的油灯灯芯结出了一朵暗红的灯花，将沈灰被岁月压弯的身影投射在斑驳的泥墙上。他重新坐回那张八仙桌旁，从怀中摸出一方粗布手绢，仔细擦拭着断尖竹篾刀上的墨迹与血痕。今夜的对峙与波澜不过是漫长棋局中的又一枚落子，而真正的生死大劫，正悄无声息地向着既定的终点奔涌而去。步步杀机。刀锋微寒。油灯如豆。"

	p11 := "老人合上双目。掌心覆在盲杖的兽首握柄上。屋顶上的瓦缝间渗漏下几滴冰冷的雨珠，滴落在脚边的黄泥盆里，发出叮咚清响。这声音在寂静的夜里显得格外空灵。只要这间铺子的火盆不灭，城隍庙前的阴阳界限便不可撼动，这方寸之间，便是属于沈灰一人的铁律天地。风止。更深。滴水穿石。心境如水。"

	p12 := "城隍庙外老树下的枯叶被积水泡透，烂成一地泥泞。沈灰坐在八仙桌前听着更鼓敲过三更。街面上除了雨滴摔在瓦片上的碎响，再无杂音。他的手指在竹篾上摩挲，感知着篾丝微妙的柔韧弧度。这是第四十个年头。也是最后一段还债的归途。世间诸事皆有定价，不论是腰缠万贯的豪绅巨贾，还是苟延残喘的市井布衣，在迈过那条浸透了桐油与朱砂的门槛石之后，留在城隍庙因果账册上的都不过是一笔不可更改的朱砂印记。他深深吸了一口微凉的夜气，手腕微翻，断尖竹篾刀在黑暗中划出一道沉静的弧光。落定。三更已至。黑夜无涯。"

	p13 := "老旧的城隍庙后墙长满了墨绿色的厚厚青苔。泥水从残破的滴水兽嘴里吐出来，打在石阶上，发出低沉有力的笃笃声响。沈灰在黑暗中听了整整一夜。每一下声响，都对应着心跳的节律。扎纸人不仅是扎骨架，更是在为虚无的亡魂塑形聚气。只要骨架正，纸面平，哪怕是阎罗殿前的恶鬼，也得遵从这三尺之地的凡人规矩。心正骨立。天道酬勤。黑暗深沉。曙光未现。"

	p14 := "风起。檐下风铃大作。当年跟随老庙祝修行的那些岁月在脑海中一一闪过，深巷里的每一座旧宅院、每一口水井、每一方石碾，都曾经发生过让人扼腕叹息的生死离合，但唯有城隍庙前的功德箱与纸灰池，始终如一地收敛着红尘里无处安放的贪嗔痴恨。劫数有常。心若冰清。长夜将晓。万象归真。"

	return []string{p1, p2, p3, p4, p5, pCustom1, pCustom2, pCustom3, p6, p7, p8, p9, p10, p11, p12, p13, p14}
}
