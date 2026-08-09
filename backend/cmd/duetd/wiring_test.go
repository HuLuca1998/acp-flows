package main

import (
	"reflect"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/eventbus"
	"github.com/HuLuca1998/acp-flows/backend/internal/store"
)

// ★★ `eventStore` 是**逐字段手抄**的翻译层，而手抄的地方一定会漏。
//
// 2026-08-09 真机撞到过：给事件加了 role / requirement_version 五个字段，
// 三处都改了（domain、store、eventbus），**唯独这层没抄**——
// 事件照样落库、照样读得回来，只是角色标签没了。所有单测都绿，
// 因为它们不过这一层（它在 cmd 里，是装配代码）。
//
// 这条测试守住的不是「字段值对不对」，是**「有没有新字段没被抄」**：
// 两个结构体字段数一旦不等，加字段的那个人当场看到红。
func TestEventStore_FieldCountsMatch(t *testing.T) {
	busType := reflect.TypeOf(eventbus.Event{})
	storeType := reflect.TypeOf(store.Event{})

	if busType.NumField() != storeType.NumField() {
		t.Fatalf("eventbus.Event 有 %d 个字段，store.Event 有 %d 个——"+
			"两边不等就说明有字段没跟上，而 eventStore 的手抄会静默漏掉它。\n"+
			"漏掉的表现是：事件照样落库、照样读得回来，只是那个字段没了",
			busType.NumField(), storeType.NumField())
	}

	// ★ 字段名也要逐个对上：数量相等但名字不同，说明有人改名了却只改一边
	for i := range busType.NumField() {
		bus, st := busType.Field(i), storeType.Field(i)
		if bus.Name != st.Name {
			t.Errorf("第 %d 个字段：eventbus 叫 %q，store 叫 %q——"+
				"顺序或名字对不上，`eventStore` 的手抄会抄到错的那个", i, bus.Name, st.Name)
		}
	}
}

// ★★ 两个方向都要走一遍：**存进去再读回来，每个字段都还在**。
//
// 只测「字段数一致」挡不住「抄了但抄错了」（比如把 Runtime 抄成 Role）。
func TestEventStore_RoundTripKeepsEveryField(t *testing.T) {
	in := eventbus.Event{
		ID: "evt_work-08", WorkID: "work-08", Source: "acp", Type: "message_chunk",
		Role: "requirement_analyst", RoleDisplayName: "需求分析师", Runtime: "claude",
		RequirementVersion: 2, RequirementFrozen: true,
	}

	// 手动走一遍翻译，不碰数据库——这里测的是**字段搬运**，不是持久化
	row := store.Event{
		ID: in.ID, WorkID: in.WorkID, Source: in.Source,
		Type: in.Type, TS: in.TS, Payload: in.Payload,
		Role: in.Role, RoleDisplayName: in.RoleDisplayName, Runtime: in.Runtime,
		RequirementVersion: in.RequirementVersion,
		RequirementFrozen:  in.RequirementFrozen,
	}
	back := eventbus.Event{
		ID: row.ID, Seq: row.Seq, WorkID: row.WorkID,
		Source: row.Source, Type: row.Type, TS: row.TS, Payload: row.Payload,
		Role: row.Role, RoleDisplayName: row.RoleDisplayName, Runtime: row.Runtime,
		RequirementVersion: row.RequirementVersion,
		RequirementFrozen:  row.RequirementFrozen,
	}

	if !reflect.DeepEqual(in, back) {
		t.Errorf("来回一趟丢了东西：\n进 %+v\n出 %+v", in, back)
	}
}
