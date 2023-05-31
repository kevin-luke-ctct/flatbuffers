/*
 * Copyright 2014 Google Inc. All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	mygame "MyGame"          // refers to generated code
	example "MyGame/Example" // refers to generated code
	pizza "Pizza"
	"encoding/json"
	optional_scalars "optional_scalars" // refers to generated code
	order "order"

	"bytes"
	"flag"
	"fmt"
	"os"
	"reflect"
	"sort"
	"testing"
	"testing/quick"

	flatbuffers "github.com/google/flatbuffers/go"
	"github.com/xeipuuv/gojsonschema"
)

var (
	cppData, javaData, outData string
	fuzz                       bool
	fuzzFields, fuzzObjects    int
)

func init() {
	flag.StringVar(&cppData, "cpp_data", "",
		"location of monsterdata_test.mon to verify against (required)")
	flag.StringVar(&javaData, "java_data", "",
		"location of monsterdata_java_wire.mon to verify against (optional)")
	flag.StringVar(&outData, "out_data", "",
		"location to write generated Go data")
	flag.BoolVar(&fuzz, "fuzz", false, "perform fuzzing")
	flag.IntVar(&fuzzFields, "fuzz_fields", 4, "fields per fuzzer object")
	flag.IntVar(&fuzzObjects, "fuzz_objects", 10000,
		"number of fuzzer objects (higher is slower and more thorough")
}

// Store specific byte patterns in these variables for the fuzzer. These
// values are taken verbatim from the C++ function FuzzTest1.
var (
	overflowingInt32Val = flatbuffers.GetInt32([]byte{0x83, 0x33, 0x33, 0x33})
	overflowingInt64Val = flatbuffers.GetInt64([]byte{0x84, 0x44, 0x44, 0x44, 0x44, 0x44, 0x44, 0x44})
)

func TestMain(m *testing.M) {
	flag.Parse()
	if cppData == "" {
		fmt.Fprintf(os.Stderr, "cpp_data argument is required\n")
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// TestTextParsing test if text parsing works with object API.
func TestTextParsing(t *testing.T) {
	expectedMonster := example.MonsterT{
		Mana: 42,
		Name: "foo",
		Test: &example.AnyT{
			Type: example.AnyMonster,
			Value: &example.MonsterT{
				Mana: 43,
				Name: "foo2",
			},
		},
		LongEnumNormalDefault: example.LongEnumLongTwo,
	}

	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(expectedMonster); err != nil {
		t.Fatal(err)
	}

	var monster example.MonsterT
	if err := json.NewDecoder(buf).Decode(&monster); err != nil {
		t.Fatal(err)
	}

	if monster.Mana != expectedMonster.Mana {
		t.Fatal("wrong mana:", monster.Mana)
	}
	if monster.Name != expectedMonster.Name {
		t.Fatal("wrong name:", monster.Name)
	}
	if monster.LongEnumNormalDefault != expectedMonster.LongEnumNormalDefault {
		t.Fatal("wrong enum:", monster.LongEnumNormalDefault)
	}

	if monster.Test.Value.(*example.MonsterT).Mana != expectedMonster.Test.Value.(*example.MonsterT).Mana {
		t.Fatal("wrong Test.Value.mana:", monster.Test.Value.(*example.MonsterT).Mana)
	}

	if monster.Test.Value.(*example.MonsterT).Name != expectedMonster.Test.Value.(*example.MonsterT).Name {
		t.Fatal("wrong Test.Value.name:", monster.Test.Value.(*example.MonsterT).Name)
	}

	expectedMonster = example.MonsterT{
		Mana: 42,
		Name: "foo",
		Test: &example.AnyT{
			Type: example.AnyTestSimpleTableWithEnum,
			Value: &example.TestSimpleTableWithEnumT{
				Color: example.ColorBlue,
			},
		},
		LongEnumNormalDefault: example.LongEnumLongTwo,
	}

	buf = new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(expectedMonster); err != nil {
		t.Fatal(err)
	}

	monster = example.MonsterT{}
	if err := json.NewDecoder(buf).Decode(&monster); err != nil {
		t.Fatal(err)
	}

	if monster.Test.Type != expectedMonster.Test.Type {
		t.Fatal("wrong Test.Type:", monster.Test.Type)
	}

	if monster.Test.Value.(*example.TestSimpleTableWithEnumT).Color != expectedMonster.Test.Value.(*example.TestSimpleTableWithEnumT).Color {
		t.Fatal("wrong Test.Type:", monster.Test.Value.(*example.TestSimpleTableWithEnumT).Color)
	}
}

func TestTextParsingValidateAgainstJsonSchema(t *testing.T) {
	expectedMonster := example.MonsterT{
		Pos:                    &example.Vec3T{X: 0.1, Y: 0.2, Z: 0.3, Test1: 0.4, Test2: example.ColorRed, Test3: &example.TestT{}},
		Mana:                   42,
		Hp:                     10,
		Name:                   "foo",
		Color:                  example.ColorBlue,
		LongEnumNormalDefault:  example.LongEnumLongTwo,
		LongEnumNonEnumDefault: example.LongEnumLongTwo,
		Flex:                   []byte{1, 2, 3},
		VectorOfLongs: []int64{
			1,
			2,
			3,
		},
		VectorOfDoubles: []float64{
			0.1,
			0.2,
			0.3,
		},
		Test5: []*example.TestT{
			&example.TestT{
				A: 1,
				B: 2,
			},
			&example.TestT{
				A: 3,
				B: 4,
			},
		},
	}

	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(expectedMonster); err != nil {
		t.Fatal(err)
	}

	var monster example.MonsterT
	if err := json.NewDecoder(bytes.NewBuffer(buf.Bytes())).Decode(&monster); err != nil {
		t.Fatal(err)
	}

	buf2 := new(bytes.Buffer)
	if err := json.NewEncoder(buf2).Encode(monster); err != nil {
		t.Fatal(err)
	}

	if string(buf.Bytes()) != string(buf2.Bytes()) {
		fmt.Printf("Expected: %s\nActual: %s\n", string(buf.Bytes()), string(buf2.Bytes()))
		t.Fatal("Monster did not encode and decode correctly")
	}

	// Check that serialized json is consistent with a generated schema
	monsterSchemaLoader := gojsonschema.NewReferenceLoader("file://../monster_test.schema.json")
	monsterJsonLoader := gojsonschema.NewBytesLoader(buf.Bytes())

	result, err := gojsonschema.Validate(monsterSchemaLoader, monsterJsonLoader)
	if err != nil {
		t.Fatal("failed to validate monster json", err)
	}

	if !result.Valid() {
		for _, desc := range result.Errors() {
			fmt.Printf("- %s\n", desc)
		}
		t.Fatal("failed to validate monster json, json invalid")
	}
}

func CheckNoNamespaceImport(fail func(string, ...interface{})) {
	const size = 13
	// Order a pizza with specific size
	builder := flatbuffers.NewBuilder(0)
	ordered_pizza := pizza.PizzaT{Size: size}
	food := order.FoodT{Pizza: &ordered_pizza}
	builder.Finish(food.Pack(builder))

	// Receive order
	received_food := order.GetRootAsFood(builder.FinishedBytes(), 0)
	received_pizza := received_food.Pizza(nil).UnPack()

	// Check if received pizza is equal to ordered pizza
	if !reflect.DeepEqual(ordered_pizza, *received_pizza) {
		fail(FailString("no namespace import", ordered_pizza, received_pizza))
	}
}

// TestAll runs all checks, failing if any errors occur.
func TestAll(t *testing.T) {
	// Verify that the Go FlatBuffers runtime library generates the
	// expected bytes (does not use any schema):
	CheckByteLayout(t.Fatalf)
	CheckMutateMethods(t.Fatalf)

	// Verify that panics are raised during exceptional conditions:
	CheckNotInObjectError(t.Fatalf)
	CheckStringIsNestedError(t.Fatalf)
	CheckByteStringIsNestedError(t.Fatalf)
	CheckStructIsNotInlineError(t.Fatalf)
	CheckFinishedBytesError(t.Fatalf)
	CheckSharedStrings(t.Fatalf)
	CheckEmptiedBuilder(t.Fatalf)

	// Verify that GetRootAs works for non-root tables
	CheckGetRootAsForNonRootTable(t.Fatalf)
	CheckTableAccessors(t.Fatalf)

	// Verify that using the generated Go code builds a buffer without
	// returning errors:
	generated, off := CheckGeneratedBuild(false, t.Fatalf)

	// Verify that the buffer generated by Go code is readable by the
	// generated Go code:
	CheckReadBuffer(generated, off, false, t.Fatalf)
	CheckMutateBuffer(generated, off, false, t.Fatalf)
	CheckObjectAPI(generated, off, false, t.Fatalf)

	// Verify that the buffer generated by C++ code is readable by the
	// generated Go code:
	monsterDataCpp, err := os.ReadFile(cppData)
	if err != nil {
		t.Fatal(err)
	}
	CheckReadBuffer(monsterDataCpp, 0, false, t.Fatalf)
	CheckMutateBuffer(monsterDataCpp, 0, false, t.Fatalf)
	CheckObjectAPI(monsterDataCpp, 0, false, t.Fatalf)

	// Verify that vtables are deduplicated when written:
	CheckVtableDeduplication(t.Fatalf)

	// Verify the enum names
	CheckEnumNames(t.Fatalf)

	// Verify enum String methods
	CheckEnumString(t.Fatalf)

	// Verify the enum values maps
	CheckEnumValues(t.Fatalf)

	// Verify that the Go code used in FlatBuffers documentation passes
	// some sanity checks:
	CheckDocExample(generated, off, t.Fatalf)

	// Check Builder.CreateByteVector
	CheckCreateByteVector(t.Fatalf)

	// Check a parent namespace import
	CheckParentNamespace(t.Fatalf)

	// Check a no namespace import
	CheckNoNamespaceImport(t.Fatalf)

	// Check size-prefixed flatbuffers
	CheckSizePrefixedBuffer(t.Fatalf)

	// Check that optional scalars works
	CheckOptionalScalars(t.Fatalf)

	// Check that getting vector element by key works
	CheckByKey(t.Fatalf)

	// If the filename of the FlatBuffers file generated by the Java test
	// is given, check that Go code can read it, and that Go code
	// generates an identical buffer when used to create the example data:
	if javaData != "" {
		monsterDataJava, err := os.ReadFile(javaData)
		if err != nil {
			t.Fatal(err)
		}
		CheckReadBuffer(monsterDataJava, 0, false, t.Fatalf)
		CheckByteEquality(generated[off:], monsterDataJava, t.Fatalf)
	}

	// Verify that various fuzzing scenarios produce a valid FlatBuffer.
	if fuzz {
		checkFuzz(fuzzFields, fuzzObjects, t.Fatalf)
	}

	// Write the generated buffer out to a file:
	err = os.WriteFile(outData, generated[off:], os.FileMode(0644))
	if err != nil {
		t.Fatal(err)
	}
}

// CheckReadBuffer checks that the given buffer is evaluated correctly
// as the example Monster.
func CheckReadBuffer(buf []byte, offset flatbuffers.UOffsetT, sizePrefix bool, fail func(string, ...interface{})) {
	// try the two ways of generating a monster
	var monster1 *example.Monster
	monster2 := &example.Monster{}

	if sizePrefix {
		monster1 = example.GetSizePrefixedRootAsMonster(buf, offset)
		flatbuffers.GetSizePrefixedRootAs(buf, offset, monster2)
	} else {
		monster1 = example.GetRootAsMonster(buf, offset)
		flatbuffers.GetRootAs(buf, offset, monster2)
	}

	for _, monster := range []*example.Monster{monster1, monster2} {
		if got := monster.Hp(); 80 != got {
			fail(FailString("hp", 80, got))
		}

		// default
		if got := monster.Mana(); 150 != got {
			fail(FailString("mana", 150, got))
		}

		if got := monster.Name(); !bytes.Equal([]byte("MyMonster"), got) {
			fail(FailString("name", "MyMonster", got))
		}

		if got := monster.Color(); example.ColorBlue != got {
			fail(FailString("color", example.ColorBlue, got))
		}

		if got := monster.Testbool(); true != got {
			fail(FailString("testbool", true, got))
		}

		// initialize a Vec3 from Pos()
		vec := new(example.Vec3)
		vec = monster.Pos(vec)
		if vec == nil {
			fail("vec3 initialization failed")
		}

		// check that new allocs equal given ones:
		vec2 := monster.Pos(nil)
		if !reflect.DeepEqual(vec, vec2) {
			fail("fresh allocation failed")
		}

		// verify the properties of the Vec3
		if got := vec.X(); float32(1.0) != got {
			fail(FailString("Pos.X", float32(1.0), got))
		}

		if got := vec.Y(); float32(2.0) != got {
			fail(FailString("Pos.Y", float32(2.0), got))
		}

		if got := vec.Z(); float32(3.0) != got {
			fail(FailString("Pos.Z", float32(3.0), got))
		}

		if got := vec.Test1(); float64(3.0) != got {
			fail(FailString("Pos.Test1", float64(3.0), got))
		}

		if got := vec.Test2(); example.ColorGreen != got {
			fail(FailString("Pos.Test2", example.ColorGreen, got))
		}

		// initialize a Test from Test3(...)
		t := new(example.Test)
		t = vec.Test3(t)
		if t == nil {
			fail("vec.Test3(&t) failed")
		}

		// check that new allocs equal given ones:
		t2 := vec.Test3(nil)
		if !reflect.DeepEqual(t, t2) {
			fail("fresh allocation failed")
		}

		// verify the properties of the Test
		if got := t.A(); int16(5) != got {
			fail(FailString("t.A()", int16(5), got))
		}

		if got := t.B(); int8(6) != got {
			fail(FailString("t.B()", int8(6), got))
		}

		if got := monster.TestType(); example.AnyMonster != got {
			fail(FailString("monster.TestType()", example.AnyMonster, got))
		}

		// initialize a Table from a union field Test(...)
		var table2 flatbuffers.Table
		if ok := monster.Test(&table2); !ok {
			fail("monster.Test(&monster2) failed")
		}

		// initialize a Monster from the Table from the union
		var monster2 example.Monster
		monster2.Init(table2.Bytes, table2.Pos)

		if got := monster2.Name(); !bytes.Equal([]byte("Fred"), got) {
			fail(FailString("monster2.Name()", "Fred", got))
		}

		inventorySlice := monster.InventoryBytes()
		if len(inventorySlice) != monster.InventoryLength() {
			fail(FailString("len(monster.InventoryBytes) != monster.InventoryLength", len(inventorySlice), monster.InventoryLength()))
		}

		if got := monster.InventoryLength(); 5 != got {
			fail(FailString("monster.InventoryLength", 5, got))
		}

		invsum := 0
		l := monster.InventoryLength()
		for i := 0; i < l; i++ {
			v := monster.Inventory(i)
			if v != inventorySlice[i] {
				fail(FailString("monster inventory slice[i] != Inventory(i)", v, inventorySlice[i]))
			}
			invsum += int(v)
		}
		if invsum != 10 {
			fail(FailString("monster inventory sum", 10, invsum))
		}

		if got := monster.Test4Length(); 2 != got {
			fail(FailString("monster.Test4Length()", 2, got))
		}

		var test0 example.Test
		ok := monster.Test4(&test0, 0)
		if !ok {
			fail(FailString("monster.Test4(&test0, 0)", true, ok))
		}

		var test1 example.Test
		ok = monster.Test4(&test1, 1)
		if !ok {
			fail(FailString("monster.Test4(&test1, 1)", true, ok))
		}

		// the position of test0 and test1 are swapped in monsterdata_java_wire
		// and monsterdata_test_wire, so ignore ordering
		v0 := test0.A()
		v1 := test0.B()
		v2 := test1.A()
		v3 := test1.B()
		sum := int(v0) + int(v1) + int(v2) + int(v3)

		if 100 != sum {
			fail(FailString("test0 and test1 sum", 100, sum))
		}

		if got := monster.TestarrayofstringLength(); 2 != got {
			fail(FailString("Testarrayofstring length", 2, got))
		}

		if got := monster.Testarrayofstring(0); !bytes.Equal([]byte("test1"), got) {
			fail(FailString("Testarrayofstring(0)", "test1", got))
		}

		if got := monster.Testarrayofstring(1); !bytes.Equal([]byte("test2"), got) {
			fail(FailString("Testarrayofstring(1)", "test2", got))
		}
	}
}

// CheckMutateBuffer checks that the given buffer can be mutated correctly
// as the example Monster. Only available scalar values are mutated.
func CheckMutateBuffer(org []byte, offset flatbuffers.UOffsetT, sizePrefix bool, fail func(string, ...interface{})) {
	// make a copy to mutate
	buf := make([]byte, len(org))
	copy(buf, org)

	// load monster data from the buffer
	var monster *example.Monster
	if sizePrefix {
		monster = example.GetSizePrefixedRootAsMonster(buf, offset)
	} else {
		monster = example.GetRootAsMonster(buf, offset)
	}

	// test case struct
	type testcase struct {
		field  string
		testfn func() bool
	}

	testForOriginalValues := []testcase{
		testcase{"Hp", func() bool { return monster.Hp() == 80 }},
		testcase{"Mana", func() bool { return monster.Mana() == 150 }},
		testcase{"Testbool", func() bool { return monster.Testbool() == true }},
		testcase{"Pos.X'", func() bool { return monster.Pos(nil).X() == float32(1.0) }},
		testcase{"Pos.Y'", func() bool { return monster.Pos(nil).Y() == float32(2.0) }},
		testcase{"Pos.Z'", func() bool { return monster.Pos(nil).Z() == float32(3.0) }},
		testcase{"Pos.Test1'", func() bool { return monster.Pos(nil).Test1() == float64(3.0) }},
		testcase{"Pos.Test2'", func() bool { return monster.Pos(nil).Test2() == example.ColorGreen }},
		testcase{"Pos.Test3.A", func() bool { return monster.Pos(nil).Test3(nil).A() == int16(5) }},
		testcase{"Pos.Test3.B", func() bool { return monster.Pos(nil).Test3(nil).B() == int8(6) }},
		testcase{"Inventory[2]", func() bool { return monster.Inventory(2) == byte(2) }},
	}

	testMutability := []testcase{
		testcase{"Hp", func() bool { return monster.MutateHp(70) }},
		testcase{"Mana", func() bool { return !monster.MutateMana(140) }},
		testcase{"Testbool", func() bool { return monster.MutateTestbool(false) }},
		testcase{"Pos.X", func() bool { return monster.Pos(nil).MutateX(10.0) }},
		testcase{"Pos.Y", func() bool { return monster.Pos(nil).MutateY(20.0) }},
		testcase{"Pos.Z", func() bool { return monster.Pos(nil).MutateZ(30.0) }},
		testcase{"Pos.Test1", func() bool { return monster.Pos(nil).MutateTest1(30.0) }},
		testcase{"Pos.Test2", func() bool { return monster.Pos(nil).MutateTest2(example.ColorBlue) }},
		testcase{"Pos.Test3.A", func() bool { return monster.Pos(nil).Test3(nil).MutateA(50) }},
		testcase{"Pos.Test3.B", func() bool { return monster.Pos(nil).Test3(nil).MutateB(60) }},
		testcase{"Inventory[2]", func() bool { return monster.MutateInventory(2, 200) }},
	}

	testForMutatedValues := []testcase{
		testcase{"Hp", func() bool { return monster.Hp() == 70 }},
		testcase{"Mana", func() bool { return monster.Mana() == 150 }},
		testcase{"Testbool", func() bool { return monster.Testbool() == false }},
		testcase{"Pos.X'", func() bool { return monster.Pos(nil).X() == float32(10.0) }},
		testcase{"Pos.Y'", func() bool { return monster.Pos(nil).Y() == float32(20.0) }},
		testcase{"Pos.Z'", func() bool { return monster.Pos(nil).Z() == float32(30.0) }},
		testcase{"Pos.Test1'", func() bool { return monster.Pos(nil).Test1() == float64(30.0) }},
		testcase{"Pos.Test2'", func() bool { return monster.Pos(nil).Test2() == example.ColorBlue }},
		testcase{"Pos.Test3.A", func() bool { return monster.Pos(nil).Test3(nil).A() == int16(50) }},
		testcase{"Pos.Test3.B", func() bool { return monster.Pos(nil).Test3(nil).B() == int8(60) }},
		testcase{"Inventory[2]", func() bool { return monster.Inventory(2) == byte(200) }},
	}

	testInvalidEnumValues := []testcase{
		testcase{"Pos.Test2", func() bool { return monster.Pos(nil).MutateTest2(example.Color(20)) }},
		testcase{"Pos.Test2", func() bool { return monster.Pos(nil).Test2() == example.Color(20) }},
	}

	// make sure original values are okay
	for _, t := range testForOriginalValues {
		if !t.testfn() {
			fail("field '" + t.field + "' doesn't have the expected original value")
		}
	}

	// try to mutate fields and check mutability
	for _, t := range testMutability {
		if !t.testfn() {
			fail(FailString("field '"+t.field+"' failed mutability test", true, false))
		}
	}

	// test whether values have changed
	for _, t := range testForMutatedValues {
		if !t.testfn() {
			fail("field '" + t.field + "' doesn't have the expected mutated value")
		}
	}

	// make sure the buffer has changed
	if reflect.DeepEqual(buf, org) {
		fail("mutate buffer failed")
	}

	// To make sure the buffer has changed accordingly
	// Read data from the buffer and verify all fields
	if sizePrefix {
		monster = example.GetSizePrefixedRootAsMonster(buf, offset)
	} else {
		monster = example.GetRootAsMonster(buf, offset)
	}

	for _, t := range testForMutatedValues {
		if !t.testfn() {
			fail("field '" + t.field + "' doesn't have the expected mutated value")
		}
	}

	// a couple extra tests for "invalid" enum values, which don't correspond to
	// anything in the schema, but are allowed
	for _, t := range testInvalidEnumValues {
		if !t.testfn() {
			fail("field '" + t.field + "' doesn't work with an invalid enum value")
		}
	}

	// reverting all fields to original values should
	// re-create the original buffer. Mutate all fields
	// back to their original values and compare buffers.
	// This test is done to make sure mutations do not do
	// any unnecessary changes to the buffer.
	if sizePrefix {
		monster = example.GetSizePrefixedRootAsMonster(buf, offset)
	} else {
		monster = example.GetRootAsMonster(buf, offset)
	}

	monster.MutateHp(80)
	monster.MutateTestbool(true)
	monster.Pos(nil).MutateX(1.0)
	monster.Pos(nil).MutateY(2.0)
	monster.Pos(nil).MutateZ(3.0)
	monster.Pos(nil).MutateTest1(3.0)
	monster.Pos(nil).MutateTest2(example.ColorGreen)
	monster.Pos(nil).Test3(nil).MutateA(5)
	monster.Pos(nil).Test3(nil).MutateB(6)
	monster.MutateInventory(2, 2)

	for _, t := range testForOriginalValues {
		if !t.testfn() {
			fail("field '" + t.field + "' doesn't have the expected original value")
		}
	}

	// buffer should have original values
	if !reflect.DeepEqual(buf, org) {
		fail("revert changes failed")
	}
}

func CheckObjectAPI(buf []byte, offset flatbuffers.UOffsetT, sizePrefix bool, fail func(string, ...interface{})) {
	var monster *example.MonsterT

	if sizePrefix {
		monster = example.GetSizePrefixedRootAsMonster(buf, offset).UnPack()
	} else {
		monster = example.GetRootAsMonster(buf, offset).UnPack()
	}

	if got := monster.Hp; 80 != got {
		fail(FailString("hp", 80, got))
	}

	// default
	if got := monster.Mana; 150 != got {
		fail(FailString("mana", 150, got))
	}

	if monster.Test != nil && monster.Test.Type == example.AnyMonster {
		monster.Test.Value.(*example.MonsterT).NanDefault = 0.0
	}
	if monster.Enemy != nil {
		monster.Enemy.NanDefault = 0.0
	}
	monster.NanDefault = 0.0

	builder := flatbuffers.NewBuilder(0)
	builder.Finish(monster.Pack(builder))
	monster2 := example.GetRootAsMonster(builder.FinishedBytes(), 0).UnPack()
	if !reflect.DeepEqual(monster, monster2) {
		fail(FailString("Pack/Unpack()", monster, monster2))
	}
}

// Low level stress/fuzz test: serialize/deserialize a variety of
// different kinds of data in different combinations
func checkFuzz(fuzzFields, fuzzObjects int, fail func(string, ...interface{})) {

	// Values we're testing against: chosen to ensure no bits get chopped
	// off anywhere, and also be different from eachother.
	boolVal := true
	int8Val := int8(-127) // 0x81
	uint8Val := uint8(0xFF)
	int16Val := int16(-32222) // 0x8222
	uint16Val := uint16(0xFEEE)
	int32Val := int32(overflowingInt32Val)
	uint32Val := uint32(0xFDDDDDDD)
	int64Val := int64(overflowingInt64Val)
	uint64Val := uint64(0xFCCCCCCCCCCCCCCC)
	float32Val := float32(3.14159)
	float64Val := float64(3.14159265359)

	testValuesMax := 11 // hardcoded to the number of scalar types

	builder := flatbuffers.NewBuilder(0)
	l := NewLCG()

	objects := make([]flatbuffers.UOffsetT, fuzzObjects)

	// Generate fuzzObjects random objects each consisting of
	// fuzzFields fields, each of a random type.
	for i := 0; i < fuzzObjects; i++ {
		builder.StartObject(fuzzFields)

		for f := 0; f < fuzzFields; f++ {
			choice := l.Next() % uint32(testValuesMax)
			switch choice {
			case 0:
				builder.PrependBoolSlot(int(f), boolVal, false)
			case 1:
				builder.PrependInt8Slot(int(f), int8Val, 0)
			case 2:
				builder.PrependUint8Slot(int(f), uint8Val, 0)
			case 3:
				builder.PrependInt16Slot(int(f), int16Val, 0)
			case 4:
				builder.PrependUint16Slot(int(f), uint16Val, 0)
			case 5:
				builder.PrependInt32Slot(int(f), int32Val, 0)
			case 6:
				builder.PrependUint32Slot(int(f), uint32Val, 0)
			case 7:
				builder.PrependInt64Slot(int(f), int64Val, 0)
			case 8:
				builder.PrependUint64Slot(int(f), uint64Val, 0)
			case 9:
				builder.PrependFloat32Slot(int(f), float32Val, 0)
			case 10:
				builder.PrependFloat64Slot(int(f), float64Val, 0)
			}
		}

		off := builder.EndObject()

		// store the offset from the end of the builder buffer,
		// since it will keep growing:
		objects[i] = off
	}

	// Do some bookkeeping to generate stats on fuzzes:
	stats := map[string]int{}
	check := func(desc string, want, got interface{}) {
		stats[desc]++
		if want != got {
			fail("%s want %v got %v", desc, want, got)
		}
	}

	l = NewLCG() // Reset.

	// Test that all objects we generated are readable and return the
	// expected values. We generate random objects in the same order
	// so this is deterministic.
	for i := 0; i < fuzzObjects; i++ {

		table := &flatbuffers.Table{
			Bytes: builder.Bytes,
			Pos:   flatbuffers.UOffsetT(len(builder.Bytes)) - objects[i],
		}

		for j := 0; j < fuzzFields; j++ {
			f := flatbuffers.VOffsetT((flatbuffers.VtableMetadataFields + j) * flatbuffers.SizeVOffsetT)
			choice := l.Next() % uint32(testValuesMax)

			switch choice {
			case 0:
				check("bool", boolVal, table.GetBoolSlot(f, false))
			case 1:
				check("int8", int8Val, table.GetInt8Slot(f, 0))
			case 2:
				check("uint8", uint8Val, table.GetUint8Slot(f, 0))
			case 3:
				check("int16", int16Val, table.GetInt16Slot(f, 0))
			case 4:
				check("uint16", uint16Val, table.GetUint16Slot(f, 0))
			case 5:
				check("int32", int32Val, table.GetInt32Slot(f, 0))
			case 6:
				check("uint32", uint32Val, table.GetUint32Slot(f, 0))
			case 7:
				check("int64", int64Val, table.GetInt64Slot(f, 0))
			case 8:
				check("uint64", uint64Val, table.GetUint64Slot(f, 0))
			case 9:
				check("float32", float32Val, table.GetFloat32Slot(f, 0))
			case 10:
				check("float64", float64Val, table.GetFloat64Slot(f, 0))
			}
		}
	}

	// If enough checks were made, verify that all scalar types were used:
	if fuzzFields*fuzzObjects >= testValuesMax {
		if len(stats) != testValuesMax {
			fail("fuzzing failed to test all scalar types")
		}
	}

	// Print some counts, if needed:
	if testing.Verbose() {
		if fuzzFields == 0 || fuzzObjects == 0 {
			fmt.Printf("fuzz\tfields: %d\tobjects: %d\t[none]\t%d\n",
				fuzzFields, fuzzObjects, 0)
		} else {
			keys := make([]string, 0, len(stats))
			for k := range stats {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Printf("fuzz\tfields: %d\tobjects: %d\t%s\t%d\n",
					fuzzFields, fuzzObjects, k, stats[k])
			}
		}
	}

	return
}

// FailString makes a message for when expectations differ from reality.
func FailString(name string, want, got interface{}) string {
	return fmt.Sprintf("bad %s: want %#v got %#v", name, want, got)
}

// CheckByteLayout verifies the bytes of a Builder in various scenarios.
func CheckByteLayout(fail func(string, ...interface{})) {
	var b *flatbuffers.Builder

	var i int
	check := func(want []byte) {
		i++
		got := b.Bytes[b.Head():]
		if !bytes.Equal(want, got) {
			fail("case %d: want\n%v\nbut got\n%v\n", i, want, got)
		}
	}

	// test 1: numbers

	b = flatbuffers.NewBuilder(0)
	check([]byte{})
	b.PrependBool(true)
	check([]byte{1})
	b.PrependInt8(-127)
	check([]byte{129, 1})
	b.PrependUint8(255)
	check([]byte{255, 129, 1})
	b.PrependInt16(-32222)
	check([]byte{0x22, 0x82, 0, 255, 129, 1}) // first pad
	b.PrependUint16(0xFEEE)
	check([]byte{0xEE, 0xFE, 0x22, 0x82, 0, 255, 129, 1}) // no pad this time
	b.PrependInt32(-53687092)
	check([]byte{204, 204, 204, 252, 0xEE, 0xFE, 0x22, 0x82, 0, 255, 129, 1})
	b.PrependUint32(0x98765432)
	check([]byte{0x32, 0x54, 0x76, 0x98, 204, 204, 204, 252, 0xEE, 0xFE, 0x22, 0x82, 0, 255, 129, 1})

	// test 1b: numbers 2

	b = flatbuffers.NewBuilder(0)
	b.PrependUint64(0x1122334455667788)
	check([]byte{0x88, 0x77, 0x66, 0x55, 0x44, 0x33, 0x22, 0x11})

	// test 2: 1xbyte vector

	b = flatbuffers.NewBuilder(0)
	check([]byte{})
	b.StartVector(flatbuffers.SizeByte, 1, 1)
	check([]byte{0, 0, 0}) // align to 4bytes
	b.PrependByte(1)
	check([]byte{1, 0, 0, 0})
	b.EndVector(1)
	check([]byte{1, 0, 0, 0, 1, 0, 0, 0}) // padding

	// test 3: 2xbyte vector

	b = flatbuffers.NewBuilder(0)
	b.StartVector(flatbuffers.SizeByte, 2, 1)
	check([]byte{0, 0}) // align to 4bytes
	b.PrependByte(1)
	check([]byte{1, 0, 0})
	b.PrependByte(2)
	check([]byte{2, 1, 0, 0})
	b.EndVector(2)
	check([]byte{2, 0, 0, 0, 2, 1, 0, 0}) // padding

	// test 3b: 11xbyte vector matches builder size

	b = flatbuffers.NewBuilder(12)
	b.StartVector(flatbuffers.SizeByte, 8, 1)
	start := []byte{}
	check(start)
	for i := 1; i < 12; i++ {
		b.PrependByte(byte(i))
		start = append([]byte{byte(i)}, start...)
		check(start)
	}
	b.EndVector(8)
	check(append([]byte{8, 0, 0, 0}, start...))

	// test 4: 1xuint16 vector

	b = flatbuffers.NewBuilder(0)
	b.StartVector(flatbuffers.SizeUint16, 1, 1)
	check([]byte{0, 0}) // align to 4bytes
	b.PrependUint16(1)
	check([]byte{1, 0, 0, 0})
	b.EndVector(1)
	check([]byte{1, 0, 0, 0, 1, 0, 0, 0}) // padding

	// test 5: 2xuint16 vector

	b = flatbuffers.NewBuilder(0)
	b.StartVector(flatbuffers.SizeUint16, 2, 1)
	check([]byte{}) // align to 4bytes
	b.PrependUint16(0xABCD)
	check([]byte{0xCD, 0xAB})
	b.PrependUint16(0xDCBA)
	check([]byte{0xBA, 0xDC, 0xCD, 0xAB})
	b.EndVector(2)
	check([]byte{2, 0, 0, 0, 0xBA, 0xDC, 0xCD, 0xAB})

	// test 6: CreateString

	b = flatbuffers.NewBuilder(0)
	b.CreateString("foo")
	check([]byte{3, 0, 0, 0, 'f', 'o', 'o', 0}) // 0-terminated, no pad
	b.CreateString("moop")
	check([]byte{4, 0, 0, 0, 'm', 'o', 'o', 'p', 0, 0, 0, 0, // 0-terminated, 3-byte pad
		3, 0, 0, 0, 'f', 'o', 'o', 0})

	// test 6b: CreateString unicode

	b = flatbuffers.NewBuilder(0)
	// These characters are chinese from blog.golang.org/strings
	// We use escape codes here so that editors without unicode support
	// aren't bothered:
	uni_str := "\u65e5\u672c\u8a9e"
	b.CreateString(uni_str)
	check([]byte{9, 0, 0, 0, 230, 151, 165, 230, 156, 172, 232, 170, 158, 0, //  null-terminated, 2-byte pad
		0, 0})

	// test 6c: CreateByteString

	b = flatbuffers.NewBuilder(0)
	b.CreateByteString([]byte("foo"))
	check([]byte{3, 0, 0, 0, 'f', 'o', 'o', 0}) // 0-terminated, no pad
	b.CreateByteString([]byte("moop"))
	check([]byte{4, 0, 0, 0, 'm', 'o', 'o', 'p', 0, 0, 0, 0, // 0-terminated, 3-byte pad
		3, 0, 0, 0, 'f', 'o', 'o', 0})

	// test 7: empty vtable
	b = flatbuffers.NewBuilder(0)
	b.StartObject(0)
	check([]byte{})
	b.EndObject()
	check([]byte{4, 0, 4, 0, 4, 0, 0, 0})

	// test 8: vtable with one true bool
	b = flatbuffers.NewBuilder(0)
	check([]byte{})
	b.StartObject(1)
	check([]byte{})
	b.PrependBoolSlot(0, true, false)
	b.EndObject()
	check([]byte{
		6, 0, // vtable bytes
		8, 0, // length of object including vtable offset
		7, 0, // start of bool value
		6, 0, 0, 0, // offset for start of vtable (int32)
		0, 0, 0, // padded to 4 bytes
		1, // bool value
	})

	// test 9: vtable with one default bool
	b = flatbuffers.NewBuilder(0)
	check([]byte{})
	b.StartObject(1)
	check([]byte{})
	b.PrependBoolSlot(0, false, false)
	b.EndObject()
	check([]byte{
		4, 0, // vtable bytes
		4, 0, // end of object from here
		// entry 1 is zero and not stored.
		4, 0, 0, 0, // offset for start of vtable (int32)
	})

	// test 10: vtable with one int16
	b = flatbuffers.NewBuilder(0)
	b.StartObject(1)
	b.PrependInt16Slot(0, 0x789A, 0)
	b.EndObject()
	check([]byte{
		6, 0, // vtable bytes
		8, 0, // end of object from here
		6, 0, // offset to value
		6, 0, 0, 0, // offset for start of vtable (int32)
		0, 0, // padding to 4 bytes
		0x9A, 0x78,
	})

	// test 11: vtable with two int16
	b = flatbuffers.NewBuilder(0)
	b.StartObject(2)
	b.PrependInt16Slot(0, 0x3456, 0)
	b.PrependInt16Slot(1, 0x789A, 0)
	b.EndObject()
	check([]byte{
		8, 0, // vtable bytes
		8, 0, // end of object from here
		6, 0, // offset to value 0
		4, 0, // offset to value 1
		8, 0, 0, 0, // offset for start of vtable (int32)
		0x9A, 0x78, // value 1
		0x56, 0x34, // value 0
	})

	// test 12: vtable with int16 and bool
	b = flatbuffers.NewBuilder(0)
	b.StartObject(2)
	b.PrependInt16Slot(0, 0x3456, 0)
	b.PrependBoolSlot(1, true, false)
	b.EndObject()
	check([]byte{
		8, 0, // vtable bytes
		8, 0, // end of object from here
		6, 0, // offset to value 0
		5, 0, // offset to value 1
		8, 0, 0, 0, // offset for start of vtable (int32)
		0,          // padding
		1,          // value 1
		0x56, 0x34, // value 0
	})

	// test 12: vtable with empty vector
	b = flatbuffers.NewBuilder(0)
	b.StartVector(flatbuffers.SizeByte, 0, 1)
	vecend := b.EndVector(0)
	b.StartObject(1)
	b.PrependUOffsetTSlot(0, vecend, 0)
	b.EndObject()
	check([]byte{
		6, 0, // vtable bytes
		8, 0,
		4, 0, // offset to vector offset
		6, 0, 0, 0, // offset for start of vtable (int32)
		4, 0, 0, 0,
		0, 0, 0, 0, // length of vector (not in struct)
	})

	// test 12b: vtable with empty vector of byte and some scalars
	b = flatbuffers.NewBuilder(0)
	b.StartVector(flatbuffers.SizeByte, 0, 1)
	vecend = b.EndVector(0)
	b.StartObject(2)
	b.PrependInt16Slot(0, 55, 0)
	b.PrependUOffsetTSlot(1, vecend, 0)
	b.EndObject()
	check([]byte{
		8, 0, // vtable bytes
		12, 0,
		10, 0, // offset to value 0
		4, 0, // offset to vector offset
		8, 0, 0, 0, // vtable loc
		8, 0, 0, 0, // value 1
		0, 0, 55, 0, // value 0

		0, 0, 0, 0, // length of vector (not in struct)
	})

	// test 13: vtable with 1 int16 and 2-vector of int16
	b = flatbuffers.NewBuilder(0)
	b.StartVector(flatbuffers.SizeInt16, 2, 1)
	b.PrependInt16(0x1234)
	b.PrependInt16(0x5678)
	vecend = b.EndVector(2)
	b.StartObject(2)
	b.PrependUOffsetTSlot(1, vecend, 0)
	b.PrependInt16Slot(0, 55, 0)
	b.EndObject()
	check([]byte{
		8, 0, // vtable bytes
		12, 0, // length of object
		6, 0, // start of value 0 from end of vtable
		8, 0, // start of value 1 from end of buffer
		8, 0, 0, 0, // offset for start of vtable (int32)
		0, 0, // padding
		55, 0, // value 0
		4, 0, 0, 0, // vector position from here
		2, 0, 0, 0, // length of vector (uint32)
		0x78, 0x56, // vector value 1
		0x34, 0x12, // vector value 0
	})

	// test 14: vtable with 1 struct of 1 int8, 1 int16, 1 int32
	b = flatbuffers.NewBuilder(0)
	b.StartObject(1)
	b.Prep(4+4+4, 0)
	b.PrependInt8(55)
	b.Pad(3)
	b.PrependInt16(0x1234)
	b.Pad(2)
	b.PrependInt32(0x12345678)
	structStart := b.Offset()
	b.PrependStructSlot(0, structStart, 0)
	b.EndObject()
	check([]byte{
		6, 0, // vtable bytes
		16, 0, // end of object from here
		4, 0, // start of struct from here
		6, 0, 0, 0, // offset for start of vtable (int32)
		0x78, 0x56, 0x34, 0x12, // value 2
		0, 0, // padding
		0x34, 0x12, // value 1
		0, 0, 0, // padding
		55, // value 0
	})

	// test 15: vtable with 1 vector of 2 struct of 2 int8
	b = flatbuffers.NewBuilder(0)
	b.StartVector(flatbuffers.SizeInt8*2, 2, 1)
	b.PrependInt8(33)
	b.PrependInt8(44)
	b.PrependInt8(55)
	b.PrependInt8(66)
	vecend = b.EndVector(2)
	b.StartObject(1)
	b.PrependUOffsetTSlot(0, vecend, 0)
	b.EndObject()
	check([]byte{
		6, 0, // vtable bytes
		8, 0,
		4, 0, // offset of vector offset
		6, 0, 0, 0, // offset for start of vtable (int32)
		4, 0, 0, 0, // vector start offset

		2, 0, 0, 0, // vector length
		66, // vector value 1,1
		55, // vector value 1,0
		44, // vector value 0,1
		33, // vector value 0,0
	})

	// test 16: table with some elements
	b = flatbuffers.NewBuilder(0)
	b.StartObject(2)
	b.PrependInt8Slot(0, 33, 0)
	b.PrependInt16Slot(1, 66, 0)
	off := b.EndObject()
	b.Finish(off)

	check([]byte{
		12, 0, 0, 0, // root of table: points to vtable offset

		8, 0, // vtable bytes
		8, 0, // end of object from here
		7, 0, // start of value 0
		4, 0, // start of value 1

		8, 0, 0, 0, // offset for start of vtable (int32)

		66, 0, // value 1
		0,  // padding
		33, // value 0
	})

	// test 17: one unfinished table and one finished table
	b = flatbuffers.NewBuilder(0)
	b.StartObject(2)
	b.PrependInt8Slot(0, 33, 0)
	b.PrependInt8Slot(1, 44, 0)
	off = b.EndObject()
	b.Finish(off)

	b.StartObject(3)
	b.PrependInt8Slot(0, 55, 0)
	b.PrependInt8Slot(1, 66, 0)
	b.PrependInt8Slot(2, 77, 0)
	off = b.EndObject()
	b.Finish(off)

	check([]byte{
		16, 0, 0, 0, // root of table: points to object
		0, 0, // padding

		10, 0, // vtable bytes
		8, 0, // size of object
		7, 0, // start of value 0
		6, 0, // start of value 1
		5, 0, // start of value 2
		10, 0, 0, 0, // offset for start of vtable (int32)
		0,  // padding
		77, // value 2
		66, // value 1
		55, // value 0

		12, 0, 0, 0, // root of table: points to object

		8, 0, // vtable bytes
		8, 0, // size of object
		7, 0, // start of value 0
		6, 0, // start of value 1
		8, 0, 0, 0, // offset for start of vtable (int32)
		0, 0, // padding
		44, // value 1
		33, // value 0
	})

	// test 18: a bunch of bools
	b = flatbuffers.NewBuilder(0)
	b.StartObject(8)
	b.PrependBoolSlot(0, true, false)
	b.PrependBoolSlot(1, true, false)
	b.PrependBoolSlot(2, true, false)
	b.PrependBoolSlot(3, true, false)
	b.PrependBoolSlot(4, true, false)
	b.PrependBoolSlot(5, true, false)
	b.PrependBoolSlot(6, true, false)
	b.PrependBoolSlot(7, true, false)
	off = b.EndObject()
	b.Finish(off)

	check([]byte{
		24, 0, 0, 0, // root of table: points to vtable offset

		20, 0, // vtable bytes
		12, 0, // size of object
		11, 0, // start of value 0
		10, 0, // start of value 1
		9, 0, // start of value 2
		8, 0, // start of value 3
		7, 0, // start of value 4
		6, 0, // start of value 5
		5, 0, // start of value 6
		4, 0, // start of value 7
		20, 0, 0, 0, // vtable offset

		1, // value 7
		1, // value 6
		1, // value 5
		1, // value 4
		1, // value 3
		1, // value 2
		1, // value 1
		1, // value 0
	})

	// test 19: three bools
	b = flatbuffers.NewBuilder(0)
	b.StartObject(3)
	b.PrependBoolSlot(0, true, false)
	b.PrependBoolSlot(1, true, false)
	b.PrependBoolSlot(2, true, false)
	off = b.EndObject()
	b.Finish(off)

	check([]byte{
		16, 0, 0, 0, // root of table: points to vtable offset

		0, 0, // padding

		10, 0, // vtable bytes
		8, 0, // size of object
		7, 0, // start of value 0
		6, 0, // start of value 1
		5, 0, // start of value 2
		10, 0, 0, 0, // vtable offset from here

		0, // padding
		1, // value 2
		1, // value 1
		1, // value 0
	})

	// test 20: some floats
	b = flatbuffers.NewBuilder(0)
	b.StartObject(1)
	b.PrependFloat32Slot(0, 1.0, 0.0)
	off = b.EndObject()

	check([]byte{
		6, 0, // vtable bytes
		8, 0, // size of object
		4, 0, // start of value 0
		6, 0, 0, 0, // vtable offset

		0, 0, 128, 63, // value 0
	})
}

// CheckManualBuild builds a Monster manually.
func CheckManualBuild(fail func(string, ...interface{})) ([]byte, flatbuffers.UOffsetT) {
	b := flatbuffers.NewBuilder(0)
	str := b.CreateString("MyMonster")

	b.StartVector(1, 5, 1)
	b.PrependByte(4)
	b.PrependByte(3)
	b.PrependByte(2)
	b.PrependByte(1)
	b.PrependByte(0)
	inv := b.EndVector(5)

	b.StartObject(13)
	b.PrependInt16Slot(2, 20, 100)
	mon2 := b.EndObject()

	// Test4Vector
	b.StartVector(4, 2, 1)

	// Test 0
	b.Prep(2, 4)
	b.Pad(1)
	b.PlaceInt8(20)
	b.PlaceInt16(10)

	// Test 1
	b.Prep(2, 4)
	b.Pad(1)
	b.PlaceInt8(40)
	b.PlaceInt16(30)

	// end testvector
	test4 := b.EndVector(2)

	b.StartObject(13)

	// a vec3
	b.Prep(16, 32)
	b.Pad(2)
	b.Prep(2, 4)
	b.Pad(1)
	b.PlaceByte(6)
	b.PlaceInt16(5)
	b.Pad(1)
	b.PlaceByte(4)
	b.PlaceFloat64(3.0)
	b.Pad(4)
	b.PlaceFloat32(3.0)
	b.PlaceFloat32(2.0)
	b.PlaceFloat32(1.0)
	vec3Loc := b.Offset()
	// end vec3

	b.PrependStructSlot(0, vec3Loc, 0) // vec3. noop
	b.PrependInt16Slot(2, 80, 100)     // hp
	b.PrependUOffsetTSlot(3, str, 0)
	b.PrependUOffsetTSlot(5, inv, 0) // inventory
	b.PrependByteSlot(7, 1, 0)
	b.PrependUOffsetTSlot(8, mon2, 0)
	b.PrependUOffsetTSlot(9, test4, 0)
	mon := b.EndObject()

	b.Finish(mon)

	return b.Bytes, b.Head()
}

func CheckGetRootAsForNonRootTable(fail func(string, ...interface{})) {
	b := flatbuffers.NewBuilder(0)
	str := b.CreateString("MyStat")
	example.StatStart(b)
	example.StatAddId(b, str)
	example.StatAddVal(b, 12345678)
	example.StatAddCount(b, 12345)
	stat_end := example.StatEnd(b)
	b.Finish(stat_end)

	stat := example.GetRootAsStat(b.Bytes, b.Head())

	if got := stat.Id(); !bytes.Equal([]byte("MyStat"), got) {
		fail(FailString("stat.Id()", "MyStat", got))
	}

	if got := stat.Val(); 12345678 != got {
		fail(FailString("stat.Val()", 12345678, got))
	}

	if got := stat.Count(); 12345 != got {
		fail(FailString("stat.Count()", 12345, got))
	}
}

// CheckGeneratedBuild uses generated code to build the example Monster.
func CheckGeneratedBuild(sizePrefix bool, fail func(string, ...interface{})) ([]byte, flatbuffers.UOffsetT) {
	b := flatbuffers.NewBuilder(0)
	str := b.CreateString("MyMonster")
	test1 := b.CreateString("test1")
	test2 := b.CreateString("test2")
	fred := b.CreateString("Fred")

	example.MonsterStartInventoryVector(b, 5)
	b.PrependByte(4)
	b.PrependByte(3)
	b.PrependByte(2)
	b.PrependByte(1)
	b.PrependByte(0)
	inv := b.EndVector(5)

	example.MonsterStart(b)
	example.MonsterAddName(b, fred)
	mon2 := example.MonsterEnd(b)

	example.MonsterStartTest4Vector(b, 2)
	example.CreateTest(b, 10, 20)
	example.CreateTest(b, 30, 40)
	test4 := b.EndVector(2)

	example.MonsterStartTestarrayofstringVector(b, 2)
	b.PrependUOffsetT(test2)
	b.PrependUOffsetT(test1)
	testArrayOfString := b.EndVector(2)

	example.MonsterStart(b)

	pos := example.CreateVec3(b, 1.0, 2.0, 3.0, 3.0, example.ColorGreen, 5, 6)
	example.MonsterAddPos(b, pos)

	example.MonsterAddHp(b, 80)
	example.MonsterAddName(b, str)
	example.MonsterAddTestbool(b, true)
	example.MonsterAddInventory(b, inv)
	example.MonsterAddTestType(b, 1)
	example.MonsterAddTest(b, mon2)
	example.MonsterAddTest4(b, test4)
	example.MonsterAddTestarrayofstring(b, testArrayOfString)
	mon := example.MonsterEnd(b)

	if sizePrefix {
		b.FinishSizePrefixed(mon)
	} else {
		b.Finish(mon)
	}

	return b.Bytes, b.Head()
}

// CheckTableAccessors checks that the table accessors work as expected.
func CheckTableAccessors(fail func(string, ...interface{})) {
	// test struct accessor
	b := flatbuffers.NewBuilder(0)
	pos := example.CreateVec3(b, 1.0, 2.0, 3.0, 3.0, 4, 5, 6)
	b.Finish(pos)
	vec3Bytes := b.FinishedBytes()
	vec3 := &example.Vec3{}
	flatbuffers.GetRootAs(vec3Bytes, 0, vec3)

	if bytes.Compare(vec3Bytes, vec3.Table().Bytes) != 0 {
		fail("invalid vec3 table")
	}

	// test table accessor
	b = flatbuffers.NewBuilder(0)
	str := b.CreateString("MyStat")
	example.StatStart(b)
	example.StatAddId(b, str)
	example.StatAddVal(b, 12345678)
	example.StatAddCount(b, 12345)
	pos = example.StatEnd(b)
	b.Finish(pos)
	statBytes := b.FinishedBytes()
	stat := &example.Stat{}
	flatbuffers.GetRootAs(statBytes, 0, stat)

	if bytes.Compare(statBytes, stat.Table().Bytes) != 0 {
		fail("invalid stat table")
	}
}

// CheckVtableDeduplication verifies that vtables are deduplicated.
func CheckVtableDeduplication(fail func(string, ...interface{})) {
	b := flatbuffers.NewBuilder(0)

	b.StartObject(4)
	b.PrependByteSlot(0, 0, 0)
	b.PrependByteSlot(1, 11, 0)
	b.PrependByteSlot(2, 22, 0)
	b.PrependInt16Slot(3, 33, 0)
	obj0 := b.EndObject()

	b.StartObject(4)
	b.PrependByteSlot(0, 0, 0)
	b.PrependByteSlot(1, 44, 0)
	b.PrependByteSlot(2, 55, 0)
	b.PrependInt16Slot(3, 66, 0)
	obj1 := b.EndObject()

	b.StartObject(4)
	b.PrependByteSlot(0, 0, 0)
	b.PrependByteSlot(1, 77, 0)
	b.PrependByteSlot(2, 88, 0)
	b.PrependInt16Slot(3, 99, 0)
	obj2 := b.EndObject()

	got := b.Bytes[b.Head():]

	want := []byte{
		240, 255, 255, 255, // == -12. offset to dedupped vtable.
		99, 0,
		88,
		77,
		248, 255, 255, 255, // == -8. offset to dedupped vtable.
		66, 0,
		55,
		44,
		12, 0,
		8, 0,
		0, 0,
		7, 0,
		6, 0,
		4, 0,
		12, 0, 0, 0,
		33, 0,
		22,
		11,
	}

	if !bytes.Equal(want, got) {
		fail("testVtableDeduplication want:\n%d %v\nbut got:\n%d %v\n",
			len(want), want, len(got), got)
	}

	table0 := &flatbuffers.Table{Bytes: b.Bytes, Pos: flatbuffers.UOffsetT(len(b.Bytes)) - obj0}
	table1 := &flatbuffers.Table{Bytes: b.Bytes, Pos: flatbuffers.UOffsetT(len(b.Bytes)) - obj1}
	table2 := &flatbuffers.Table{Bytes: b.Bytes, Pos: flatbuffers.UOffsetT(len(b.Bytes)) - obj2}

	testTable := func(tab *flatbuffers.Table, a flatbuffers.VOffsetT, b, c, d byte) {
		// vtable size
		if got := tab.GetVOffsetTSlot(0, 0); 12 != got {
			fail("failed 0, 0: %d", got)
		}
		// object size
		if got := tab.GetVOffsetTSlot(2, 0); 8 != got {
			fail("failed 2, 0: %d", got)
		}
		// default value
		if got := tab.GetVOffsetTSlot(4, 0); a != got {
			fail("failed 4, 0: %d", got)
		}
		if got := tab.GetByteSlot(6, 0); b != got {
			fail("failed 6, 0: %d", got)
		}
		if val := tab.GetByteSlot(8, 0); c != val {
			fail("failed 8, 0: %d", got)
		}
		if got := tab.GetByteSlot(10, 0); d != got {
			fail("failed 10, 0: %d", got)
		}
	}

	testTable(table0, 0, 11, 22, 33)
	testTable(table1, 0, 44, 55, 66)
	testTable(table2, 0, 77, 88, 99)
}

// CheckNotInObjectError verifies that `EndObject` fails if not inside an
// object.
func CheckNotInObjectError(fail func(string, ...interface{})) {
	b := flatbuffers.NewBuilder(0)

	defer func() {
		r := recover()
		if r == nil {
			fail("expected panic in CheckNotInObjectError")
		}
	}()
	b.EndObject()
}

// CheckStringIsNestedError verifies that a string can not be created inside
// another object.
func CheckStringIsNestedError(fail func(string, ...interface{})) {
	b := flatbuffers.NewBuilder(0)
	b.StartObject(0)
	defer func() {
		r := recover()
		if r == nil {
			fail("expected panic in CheckStringIsNestedError")
		}
	}()
	b.CreateString("foo")
}

func CheckEmptiedBuilder(fail func(string, ...interface{})) {
	f := func(a, b string) bool {
		if a == b {
			return true
		}

		builder := flatbuffers.NewBuilder(0)

		a1 := builder.CreateSharedString(a)
		b1 := builder.CreateSharedString(b)
		builder.Reset()
		b2 := builder.CreateSharedString(b)
		a2 := builder.CreateSharedString(a)

		return !(a1 == a2 || b1 == b2)
	}
	if err := quick.Check(f, nil); err != nil {
		fail("expected different offset")
	}
}

func CheckSharedStrings(fail func(string, ...interface{})) {
	f := func(strings []string) bool {
		b := flatbuffers.NewBuilder(0)
		for _, s1 := range strings {
			for _, s2 := range strings {
				off1 := b.CreateSharedString(s1)
				off2 := b.CreateSharedString(s2)

				if (s1 == s2) && (off1 != off2) {
					return false
				}
				if (s1 != s2) && (off1 == off2) {
					return false
				}
			}
		}
		return true
	}
	if err := quick.Check(f, nil); err != nil {
		fail("expected same offset")
	}
}

// CheckByteStringIsNestedError verifies that a bytestring can not be created
// inside another object.
func CheckByteStringIsNestedError(fail func(string, ...interface{})) {
	b := flatbuffers.NewBuilder(0)
	b.StartObject(0)
	defer func() {
		r := recover()
		if r == nil {
			fail("expected panic in CheckByteStringIsNestedError")
		}
	}()
	b.CreateByteString([]byte("foo"))
}

// CheckStructIsNotInlineError verifies that writing a struct in a location
// away from where it is used will cause a panic.
func CheckStructIsNotInlineError(fail func(string, ...interface{})) {
	b := flatbuffers.NewBuilder(0)
	b.StartObject(0)
	defer func() {
		r := recover()
		if r == nil {
			fail("expected panic in CheckStructIsNotInlineError")
		}
	}()
	b.PrependStructSlot(0, 1, 0)
}

// CheckFinishedBytesError verifies that `FinishedBytes` panics if the table
// is not finished.
func CheckFinishedBytesError(fail func(string, ...interface{})) {
	b := flatbuffers.NewBuilder(0)

	defer func() {
		r := recover()
		if r == nil {
			fail("expected panic in CheckFinishedBytesError")
		}
	}()
	b.FinishedBytes()
}

// CheckEnumNames checks that the generated enum names are correct.
func CheckEnumNames(fail func(string, ...interface{})) {
	{
		want := map[example.Any]string{
			example.AnyNONE:                    "NONE",
			example.AnyMonster:                 "Monster",
			example.AnyTestSimpleTableWithEnum: "TestSimpleTableWithEnum",
			example.AnyMyGame_Example2_Monster: "MyGame_Example2_Monster",
		}
		got := example.EnumNamesAny
		if !reflect.DeepEqual(got, want) {
			fail("enum name is not equal")
		}
	}
	{
		want := map[example.Color]string{
			example.ColorRed:   "Red",
			example.ColorGreen: "Green",
			example.ColorBlue:  "Blue",
		}
		got := example.EnumNamesColor
		if !reflect.DeepEqual(got, want) {
			fail("enum name is not equal")
		}
	}
}

// CheckEnumString checks the String method on generated enum types.
func CheckEnumString(fail func(string, ...interface{})) {
	if got := example.AnyMonster.String(); got != "Monster" {
		fail("Monster.String: %q != %q", got, "Monster")
	}
	if got := fmt.Sprintf("color: %s", example.ColorGreen); got != "color: Green" {
		fail("color.String: %q != %q", got, "color: Green")
	}
}

// CheckEnumValues checks that the generated enum values maps are correct.
func CheckEnumValues(fail func(string, ...interface{})) {
	{
		want := map[string]example.Any{
			"NONE":                    example.AnyNONE,
			"Monster":                 example.AnyMonster,
			"TestSimpleTableWithEnum": example.AnyTestSimpleTableWithEnum,
			"MyGame_Example2_Monster": example.AnyMyGame_Example2_Monster,
		}
		got := example.EnumValuesAny
		if !reflect.DeepEqual(got, want) {
			fail("enum name is not equal")
		}
	}
	{
		want := map[string]example.Color{
			"Red":   example.ColorRed,
			"Green": example.ColorGreen,
			"Blue":  example.ColorBlue,
		}
		got := example.EnumValuesColor
		if !reflect.DeepEqual(got, want) {
			fail("enum name is not equal")
		}
	}
}

// CheckDocExample checks that the code given in FlatBuffers documentation
// is syntactically correct.
func CheckDocExample(buf []byte, off flatbuffers.UOffsetT, fail func(string, ...interface{})) {
	monster := example.GetRootAsMonster(buf, off)
	_ = monster.Hp()
	_ = monster.Pos(nil)
	for i := 0; i < monster.InventoryLength(); i++ {
		_ = monster.Inventory(i) // do something here
	}

	builder := flatbuffers.NewBuilder(0)

	example.MonsterStartInventoryVector(builder, 5)
	for i := 4; i >= 0; i-- {
		builder.PrependByte(byte(i))
	}
	inv := builder.EndVector(5)

	str := builder.CreateString("MyMonster")
	example.MonsterStart(builder)
	example.MonsterAddPos(builder, example.CreateVec3(builder, 1.0, 2.0, 3.0, 3.0, example.Color(4), 5, 6))
	example.MonsterAddHp(builder, 80)
	example.MonsterAddName(builder, str)
	example.MonsterAddInventory(builder, inv)
	example.MonsterAddTestType(builder, 1)
	example.MonsterAddColor(builder, example.ColorRed)
	// example.MonsterAddTest(builder, mon2)
	// example.MonsterAddTest4(builder, test4s)
	_ = example.MonsterEnd(builder)
}

func CheckCreateByteVector(fail func(string, ...interface{})) {
	raw := [30]byte{}
	for i := 0; i < len(raw); i++ {
		raw[i] = byte(i)
	}

	for size := 0; size < len(raw); size++ {
		b1 := flatbuffers.NewBuilder(0)
		b2 := flatbuffers.NewBuilder(0)
		b1.StartVector(1, size, 1)
		for i := size - 1; i >= 0; i-- {
			b1.PrependByte(raw[i])
		}
		b1.EndVector(size)
		b2.CreateByteVector(raw[:size])
		CheckByteEquality(b1.Bytes, b2.Bytes, fail)
	}
}

func CheckParentNamespace(fail func(string, ...interface{})) {
	var empty, nonempty []byte

	// create monster with an empty parent namespace field
	{
		builder := flatbuffers.NewBuilder(0)

		example.MonsterStart(builder)
		m := example.MonsterEnd(builder)
		builder.Finish(m)

		empty = make([]byte, len(builder.FinishedBytes()))
		copy(empty, builder.FinishedBytes())
	}

	// create monster with a non-empty parent namespace field
	{
		builder := flatbuffers.NewBuilder(0)
		mygame.InParentNamespaceStart(builder)
		pn := mygame.InParentNamespaceEnd(builder)

		example.MonsterStart(builder)
		example.MonsterAddParentNamespaceTest(builder, pn)
		m := example.MonsterEnd(builder)

		builder.Finish(m)

		nonempty = make([]byte, len(builder.FinishedBytes()))
		copy(nonempty, builder.FinishedBytes())
	}

	// read monster with empty parent namespace field
	{
		m := example.GetRootAsMonster(empty, 0)
		if m.ParentNamespaceTest(nil) != nil {
			fail("expected nil ParentNamespaceTest for empty field")
		}
	}

	// read monster with non-empty parent namespace field
	{
		m := example.GetRootAsMonster(nonempty, 0)
		if m.ParentNamespaceTest(nil) == nil {
			fail("expected non-nil ParentNamespaceTest for non-empty field")
		}
	}
}

func CheckSizePrefixedBuffer(fail func(string, ...interface{})) {
	// Generate a size-prefixed flatbuffer
	generated, off := CheckGeneratedBuild(true, fail)

	// Check that the size prefix is the size of monsterdata_go_wire.mon minus 4
	size := flatbuffers.GetSizePrefix(generated, off)
	if size != 220 {
		fail("mismatch between size prefix and expected size")
	}

	// Check that the buffer can be used as expected
	CheckReadBuffer(generated, off, true, fail)
	CheckMutateBuffer(generated, off, true, fail)
	CheckObjectAPI(generated, off, true, fail)

	// Write generated bfufer out to a file
	if err := os.WriteFile(outData+".sp", generated[off:], os.FileMode(0644)); err != nil {
		fail("failed to write file: %s", err)
	}
}

// Include simple random number generator to ensure results will be the
// same cross platform.
// http://en.wikipedia.org/wiki/Park%E2%80%93Miller_random_number_generator
type LCG uint32

const InitialLCGSeed = 48271

func NewLCG() *LCG {
	n := uint32(InitialLCGSeed)
	l := LCG(n)
	return &l
}

func (lcg *LCG) Reset() {
	*lcg = InitialLCGSeed
}

func (lcg *LCG) Next() uint32 {
	n := uint32((uint64(*lcg) * uint64(279470273)) % uint64(4294967291))
	*lcg = LCG(n)
	return n
}

// CheckByteEquality verifies that two byte buffers are the same.
func CheckByteEquality(a, b []byte, fail func(string, ...interface{})) {
	if !bytes.Equal(a, b) {
		fail("objects are not byte-wise equal")
	}
}

// CheckMutateMethods checks all mutate methods one by one
func CheckMutateMethods(fail func(string, ...interface{})) {
	b := flatbuffers.NewBuilder(0)
	b.StartObject(15)
	b.PrependBoolSlot(0, true, false)
	b.PrependByteSlot(1, 1, 0)
	b.PrependUint8Slot(2, 2, 0)
	b.PrependUint16Slot(3, 3, 0)
	b.PrependUint32Slot(4, 4, 0)
	b.PrependUint64Slot(5, 5, 0)
	b.PrependInt8Slot(6, 6, 0)
	b.PrependInt16Slot(7, 7, 0)
	b.PrependInt32Slot(8, 8, 0)
	b.PrependInt64Slot(9, 9, 0)
	b.PrependFloat32Slot(10, 10, 0)
	b.PrependFloat64Slot(11, 11, 0)

	b.PrependUOffsetTSlot(12, 12, 0)
	uoVal := b.Offset() - 12

	b.PrependVOffsetT(13)
	b.Slot(13)

	b.PrependSOffsetT(14)
	b.Slot(14)
	soVal := flatbuffers.SOffsetT(b.Offset() - 14)

	offset := b.EndObject()

	t := &flatbuffers.Table{
		Bytes: b.Bytes,
		Pos:   flatbuffers.UOffsetT(len(b.Bytes)) - offset,
	}

	calcVOffsetT := func(slot int) (vtableOffset flatbuffers.VOffsetT) {
		return flatbuffers.VOffsetT((flatbuffers.VtableMetadataFields + slot) * flatbuffers.SizeVOffsetT)
	}
	calcUOffsetT := func(vtableOffset flatbuffers.VOffsetT) (valueOffset flatbuffers.UOffsetT) {
		return t.Pos + flatbuffers.UOffsetT(t.Offset(vtableOffset))
	}

	type testcase struct {
		field  string
		testfn func() bool
	}

	testForOriginalValues := []testcase{
		testcase{"BoolSlot", func() bool { return t.GetBoolSlot(calcVOffsetT(0), true) == true }},
		testcase{"ByteSlot", func() bool { return t.GetByteSlot(calcVOffsetT(1), 1) == 1 }},
		testcase{"Uint8Slot", func() bool { return t.GetUint8Slot(calcVOffsetT(2), 2) == 2 }},
		testcase{"Uint16Slot", func() bool { return t.GetUint16Slot(calcVOffsetT(3), 3) == 3 }},
		testcase{"Uint32Slot", func() bool { return t.GetUint32Slot(calcVOffsetT(4), 4) == 4 }},
		testcase{"Uint64Slot", func() bool { return t.GetUint64Slot(calcVOffsetT(5), 5) == 5 }},
		testcase{"Int8Slot", func() bool { return t.GetInt8Slot(calcVOffsetT(6), 6) == 6 }},
		testcase{"Int16Slot", func() bool { return t.GetInt16Slot(calcVOffsetT(7), 7) == 7 }},
		testcase{"Int32Slot", func() bool { return t.GetInt32Slot(calcVOffsetT(8), 8) == 8 }},
		testcase{"Int64Slot", func() bool { return t.GetInt64Slot(calcVOffsetT(9), 9) == 9 }},
		testcase{"Float32Slot", func() bool { return t.GetFloat32Slot(calcVOffsetT(10), 10) == 10 }},
		testcase{"Float64Slot", func() bool { return t.GetFloat64Slot(calcVOffsetT(11), 11) == 11 }},
		testcase{"UOffsetTSlot", func() bool { return t.GetUOffsetT(calcUOffsetT(calcVOffsetT(12))) == uoVal }},
		testcase{"VOffsetTSlot", func() bool { return t.GetVOffsetT(calcUOffsetT(calcVOffsetT(13))) == 13 }},
		testcase{"SOffsetTSlot", func() bool { return t.GetSOffsetT(calcUOffsetT(calcVOffsetT(14))) == soVal }},
	}

	testMutability := []testcase{
		testcase{"BoolSlot", func() bool { return t.MutateBoolSlot(calcVOffsetT(0), false) }},
		testcase{"ByteSlot", func() bool { return t.MutateByteSlot(calcVOffsetT(1), 2) }},
		testcase{"Uint8Slot", func() bool { return t.MutateUint8Slot(calcVOffsetT(2), 4) }},
		testcase{"Uint16Slot", func() bool { return t.MutateUint16Slot(calcVOffsetT(3), 6) }},
		testcase{"Uint32Slot", func() bool { return t.MutateUint32Slot(calcVOffsetT(4), 8) }},
		testcase{"Uint64Slot", func() bool { return t.MutateUint64Slot(calcVOffsetT(5), 10) }},
		testcase{"Int8Slot", func() bool { return t.MutateInt8Slot(calcVOffsetT(6), 12) }},
		testcase{"Int16Slot", func() bool { return t.MutateInt16Slot(calcVOffsetT(7), 14) }},
		testcase{"Int32Slot", func() bool { return t.MutateInt32Slot(calcVOffsetT(8), 16) }},
		testcase{"Int64Slot", func() bool { return t.MutateInt64Slot(calcVOffsetT(9), 18) }},
		testcase{"Float32Slot", func() bool { return t.MutateFloat32Slot(calcVOffsetT(10), 20) }},
		testcase{"Float64Slot", func() bool { return t.MutateFloat64Slot(calcVOffsetT(11), 22) }},
		testcase{"UOffsetTSlot", func() bool { return t.MutateUOffsetT(calcUOffsetT(calcVOffsetT(12)), 24) }},
		testcase{"VOffsetTSlot", func() bool { return t.MutateVOffsetT(calcUOffsetT(calcVOffsetT(13)), 26) }},
		testcase{"SOffsetTSlot", func() bool { return t.MutateSOffsetT(calcUOffsetT(calcVOffsetT(14)), 28) }},
	}

	testMutabilityWithoutSlot := []testcase{
		testcase{"BoolSlot", func() bool { return t.MutateBoolSlot(calcVOffsetT(16), false) }},
		testcase{"ByteSlot", func() bool { return t.MutateByteSlot(calcVOffsetT(16), 2) }},
		testcase{"Uint8Slot", func() bool { return t.MutateUint8Slot(calcVOffsetT(16), 2) }},
		testcase{"Uint16Slot", func() bool { return t.MutateUint16Slot(calcVOffsetT(16), 2) }},
		testcase{"Uint32Slot", func() bool { return t.MutateUint32Slot(calcVOffsetT(16), 2) }},
		testcase{"Uint64Slot", func() bool { return t.MutateUint64Slot(calcVOffsetT(16), 2) }},
		testcase{"Int8Slot", func() bool { return t.MutateInt8Slot(calcVOffsetT(16), 2) }},
		testcase{"Int16Slot", func() bool { return t.MutateInt16Slot(calcVOffsetT(16), 2) }},
		testcase{"Int32Slot", func() bool { return t.MutateInt32Slot(calcVOffsetT(16), 2) }},
		testcase{"Int64Slot", func() bool { return t.MutateInt64Slot(calcVOffsetT(16), 2) }},
		testcase{"Float32Slot", func() bool { return t.MutateFloat32Slot(calcVOffsetT(16), 2) }},
		testcase{"Float64Slot", func() bool { return t.MutateFloat64Slot(calcVOffsetT(16), 2) }},
	}

	testForMutatedValues := []testcase{
		testcase{"BoolSlot", func() bool { return t.GetBoolSlot(calcVOffsetT(0), true) == false }},
		testcase{"ByteSlot", func() bool { return t.GetByteSlot(calcVOffsetT(1), 1) == 2 }},
		testcase{"Uint8Slot", func() bool { return t.GetUint8Slot(calcVOffsetT(2), 1) == 4 }},
		testcase{"Uint16Slot", func() bool { return t.GetUint16Slot(calcVOffsetT(3), 1) == 6 }},
		testcase{"Uint32Slot", func() bool { return t.GetUint32Slot(calcVOffsetT(4), 1) == 8 }},
		testcase{"Uint64Slot", func() bool { return t.GetUint64Slot(calcVOffsetT(5), 1) == 10 }},
		testcase{"Int8Slot", func() bool { return t.GetInt8Slot(calcVOffsetT(6), 1) == 12 }},
		testcase{"Int16Slot", func() bool { return t.GetInt16Slot(calcVOffsetT(7), 1) == 14 }},
		testcase{"Int32Slot", func() bool { return t.GetInt32Slot(calcVOffsetT(8), 1) == 16 }},
		testcase{"Int64Slot", func() bool { return t.GetInt64Slot(calcVOffsetT(9), 1) == 18 }},
		testcase{"Float32Slot", func() bool { return t.GetFloat32Slot(calcVOffsetT(10), 1) == 20 }},
		testcase{"Float64Slot", func() bool { return t.GetFloat64Slot(calcVOffsetT(11), 1) == 22 }},
		testcase{"UOffsetTSlot", func() bool { return t.GetUOffsetT(calcUOffsetT(calcVOffsetT(12))) == 24 }},
		testcase{"VOffsetTSlot", func() bool { return t.GetVOffsetT(calcUOffsetT(calcVOffsetT(13))) == 26 }},
		testcase{"SOffsetTSlot", func() bool { return t.GetSOffsetT(calcUOffsetT(calcVOffsetT(14))) == 28 }},
	}

	// make sure original values are okay
	for _, t := range testForOriginalValues {
		if !t.testfn() {
			fail(t.field + "' field doesn't have the expected original value")
		}
	}

	// try to mutate fields and check mutability
	for _, t := range testMutability {
		if !t.testfn() {
			fail(FailString(t.field+"' field failed mutability test", "passed", "failed"))
		}
	}

	// try to mutate fields and check mutability
	// these have wrong slots so should fail
	for _, t := range testMutabilityWithoutSlot {
		if t.testfn() {
			fail(FailString(t.field+"' field failed no slot mutability test", "failed", "passed"))
		}
	}

	// test whether values have changed
	for _, t := range testForMutatedValues {
		if !t.testfn() {
			fail(t.field + "' field doesn't have the expected mutated value")
		}
	}
}

// CheckOptionalScalars verifies against the ScalarStuff schema.
func CheckOptionalScalars(fail func(string, ...interface{})) {
	type testCase struct {
		what           string
		result, expect interface{}
	}

	makeDefaultTestCases := func(s *optional_scalars.ScalarStuff) []testCase {
		return []testCase{
			{"justI8", s.JustI8(), int8(0)},
			{"maybeI8", s.MaybeI8(), (*int8)(nil)},
			{"defaultI8", s.DefaultI8(), int8(42)},
			{"justU8", s.JustU8(), byte(0)},
			{"maybeU8", s.MaybeU8(), (*byte)(nil)},
			{"defaultU8", s.DefaultU8(), byte(42)},
			{"justI16", s.JustI16(), int16(0)},
			{"maybeI16", s.MaybeI16(), (*int16)(nil)},
			{"defaultI16", s.DefaultI16(), int16(42)},
			{"justU16", s.JustU16(), uint16(0)},
			{"maybeU16", s.MaybeU16(), (*uint16)(nil)},
			{"defaultU16", s.DefaultU16(), uint16(42)},
			{"justI32", s.JustI32(), int32(0)},
			{"maybeI32", s.MaybeI32(), (*int32)(nil)},
			{"defaultI32", s.DefaultI32(), int32(42)},
			{"justU32", s.JustU32(), uint32(0)},
			{"maybeU32", s.MaybeU32(), (*uint32)(nil)},
			{"defaultU32", s.DefaultU32(), uint32(42)},
			{"justI64", s.JustI64(), int64(0)},
			{"maybeI64", s.MaybeI64(), (*int64)(nil)},
			{"defaultI64", s.DefaultI64(), int64(42)},
			{"justU64", s.JustU64(), uint64(0)},
			{"maybeU64", s.MaybeU64(), (*uint64)(nil)},
			{"defaultU64", s.DefaultU64(), uint64(42)},
			{"justF32", s.JustF32(), float32(0)},
			{"maybeF32", s.MaybeF32(), (*float32)(nil)},
			{"defaultF32", s.DefaultF32(), float32(42)},
			{"justF64", s.JustF64(), float64(0)},
			{"maybeF64", s.MaybeF64(), (*float64)(nil)},
			{"defaultF64", s.DefaultF64(), float64(42)},
			{"justBool", s.JustBool(), false},
			{"maybeBool", s.MaybeBool(), (*bool)(nil)},
			{"defaultBool", s.DefaultBool(), true},
			{"justEnum", s.JustEnum(), optional_scalars.OptionalByte(0)},
			{"maybeEnum", s.MaybeEnum(), (*optional_scalars.OptionalByte)(nil)},
			{"defaultEnum", s.DefaultEnum(), optional_scalars.OptionalByteOne},
		}
	}

	makeAssignedTestCases := func(s *optional_scalars.ScalarStuff) []testCase {
		return []testCase{
			{"justI8", s.JustI8(), int8(5)},
			{"maybeI8", s.MaybeI8(), int8(5)},
			{"defaultI8", s.DefaultI8(), int8(5)},
			{"justU8", s.JustU8(), byte(6)},
			{"maybeU8", s.MaybeU8(), byte(6)},
			{"defaultU8", s.DefaultU8(), byte(6)},
			{"justI16", s.JustI16(), int16(7)},
			{"maybeI16", s.MaybeI16(), int16(7)},
			{"defaultI16", s.DefaultI16(), int16(7)},
			{"justU16", s.JustU16(), uint16(8)},
			{"maybeU16", s.MaybeU16(), uint16(8)},
			{"defaultU16", s.DefaultU16(), uint16(8)},
			{"justI32", s.JustI32(), int32(9)},
			{"maybeI32", s.MaybeI32(), int32(9)},
			{"defaultI32", s.DefaultI32(), int32(9)},
			{"justU32", s.JustU32(), uint32(10)},
			{"maybeU32", s.MaybeU32(), uint32(10)},
			{"defaultU32", s.DefaultU32(), uint32(10)},
			{"justI64", s.JustI64(), int64(11)},
			{"maybeI64", s.MaybeI64(), int64(11)},
			{"defaultI64", s.DefaultI64(), int64(11)},
			{"justU64", s.JustU64(), uint64(12)},
			{"maybeU64", s.MaybeU64(), uint64(12)},
			{"defaultU64", s.DefaultU64(), uint64(12)},
			{"justF32", s.JustF32(), float32(13)},
			{"maybeF32", s.MaybeF32(), float32(13)},
			{"defaultF32", s.DefaultF32(), float32(13)},
			{"justF64", s.JustF64(), float64(14)},
			{"maybeF64", s.MaybeF64(), float64(14)},
			{"defaultF64", s.DefaultF64(), float64(14)},
			{"justBool", s.JustBool(), true},
			{"maybeBool", s.MaybeBool(), true},
			{"defaultBool", s.DefaultBool(), false},
			{"justEnum", s.JustEnum(), optional_scalars.OptionalByteTwo},
			{"maybeEnum", s.MaybeEnum(), optional_scalars.OptionalByteTwo},
			{"defaultEnum", s.DefaultEnum(), optional_scalars.OptionalByteTwo},
		}
	}

	resolvePointer := func(v interface{}) interface{} {
		switch v := v.(type) {
		case *int8:
			return *v
		case *byte:
			return *v
		case *int16:
			return *v
		case *uint16:
			return *v
		case *int32:
			return *v
		case *uint32:
			return *v
		case *int64:
			return *v
		case *uint64:
			return *v
		case *float32:
			return *v
		case *float64:
			return *v
		case *bool:
			return *v
		case *optional_scalars.OptionalByte:
			return *v
		default:
			return v
		}
	}

	buildAssignedTable := func(b *flatbuffers.Builder) *optional_scalars.ScalarStuff {
		optional_scalars.ScalarStuffStart(b)
		optional_scalars.ScalarStuffAddJustI8(b, int8(5))
		optional_scalars.ScalarStuffAddMaybeI8(b, int8(5))
		optional_scalars.ScalarStuffAddDefaultI8(b, int8(5))
		optional_scalars.ScalarStuffAddJustU8(b, byte(6))
		optional_scalars.ScalarStuffAddMaybeU8(b, byte(6))
		optional_scalars.ScalarStuffAddDefaultU8(b, byte(6))
		optional_scalars.ScalarStuffAddJustI16(b, int16(7))
		optional_scalars.ScalarStuffAddMaybeI16(b, int16(7))
		optional_scalars.ScalarStuffAddDefaultI16(b, int16(7))
		optional_scalars.ScalarStuffAddJustU16(b, uint16(8))
		optional_scalars.ScalarStuffAddMaybeU16(b, uint16(8))
		optional_scalars.ScalarStuffAddDefaultU16(b, uint16(8))
		optional_scalars.ScalarStuffAddJustI32(b, int32(9))
		optional_scalars.ScalarStuffAddMaybeI32(b, int32(9))
		optional_scalars.ScalarStuffAddDefaultI32(b, int32(9))
		optional_scalars.ScalarStuffAddJustU32(b, uint32(10))
		optional_scalars.ScalarStuffAddMaybeU32(b, uint32(10))
		optional_scalars.ScalarStuffAddDefaultU32(b, uint32(10))
		optional_scalars.ScalarStuffAddJustI64(b, int64(11))
		optional_scalars.ScalarStuffAddMaybeI64(b, int64(11))
		optional_scalars.ScalarStuffAddDefaultI64(b, int64(11))
		optional_scalars.ScalarStuffAddJustU64(b, uint64(12))
		optional_scalars.ScalarStuffAddMaybeU64(b, uint64(12))
		optional_scalars.ScalarStuffAddDefaultU64(b, uint64(12))
		optional_scalars.ScalarStuffAddJustF32(b, float32(13))
		optional_scalars.ScalarStuffAddMaybeF32(b, float32(13))
		optional_scalars.ScalarStuffAddDefaultF32(b, float32(13))
		optional_scalars.ScalarStuffAddJustF64(b, float64(14))
		optional_scalars.ScalarStuffAddMaybeF64(b, float64(14))
		optional_scalars.ScalarStuffAddDefaultF64(b, float64(14))
		optional_scalars.ScalarStuffAddJustBool(b, true)
		optional_scalars.ScalarStuffAddMaybeBool(b, true)
		optional_scalars.ScalarStuffAddDefaultBool(b, false)
		optional_scalars.ScalarStuffAddJustEnum(b, optional_scalars.OptionalByteTwo)
		optional_scalars.ScalarStuffAddMaybeEnum(b, optional_scalars.OptionalByteTwo)
		optional_scalars.ScalarStuffAddDefaultEnum(b, optional_scalars.OptionalByteTwo)
		b.Finish(optional_scalars.ScalarStuffEnd(b))
		return optional_scalars.GetRootAsScalarStuff(b.FinishedBytes(), 0)
	}

	// test default values

	fbb := flatbuffers.NewBuilder(1)
	optional_scalars.ScalarStuffStart(fbb)
	fbb.Finish(optional_scalars.ScalarStuffEnd(fbb))
	ss := optional_scalars.GetRootAsScalarStuff(fbb.FinishedBytes(), 0)
	for _, tc := range makeDefaultTestCases(ss) {
		if tc.result != tc.expect {
			fail(FailString("Default ScalarStuff: "+tc.what, tc.expect, tc.result))
		}
	}

	// test assigned values
	fbb.Reset()
	ss = buildAssignedTable(fbb)
	for _, tc := range makeAssignedTestCases(ss) {
		if resolvePointer(tc.result) != tc.expect {
			fail(FailString("Assigned ScalarStuff: "+tc.what, tc.expect, tc.result))
		}
	}

	// test native object pack
	fbb.Reset()
	i8 := int8(5)
	u8 := byte(6)
	i16 := int16(7)
	u16 := uint16(8)
	i32 := int32(9)
	u32 := uint32(10)
	i64 := int64(11)
	u64 := uint64(12)
	f32 := float32(13)
	f64 := float64(14)
	b := true
	enum := optional_scalars.OptionalByteTwo
	obj := optional_scalars.ScalarStuffT{
		JustI8:      5,
		MaybeI8:     &i8,
		DefaultI8:   5,
		JustU8:      6,
		MaybeU8:     &u8,
		DefaultU8:   6,
		JustI16:     7,
		MaybeI16:    &i16,
		DefaultI16:  7,
		JustU16:     8,
		MaybeU16:    &u16,
		DefaultU16:  8,
		JustI32:     9,
		MaybeI32:    &i32,
		DefaultI32:  9,
		JustU32:     10,
		MaybeU32:    &u32,
		DefaultU32:  10,
		JustI64:     11,
		MaybeI64:    &i64,
		DefaultI64:  11,
		JustU64:     12,
		MaybeU64:    &u64,
		DefaultU64:  12,
		JustF32:     13,
		MaybeF32:    &f32,
		DefaultF32:  13,
		JustF64:     14,
		MaybeF64:    &f64,
		DefaultF64:  14,
		JustBool:    true,
		MaybeBool:   &b,
		DefaultBool: false,
		JustEnum:    optional_scalars.OptionalByteTwo,
		MaybeEnum:   &enum,
		DefaultEnum: optional_scalars.OptionalByteTwo,
	}
	fbb.Finish(obj.Pack(fbb))
	ss = optional_scalars.GetRootAsScalarStuff(fbb.FinishedBytes(), 0)
	for _, tc := range makeAssignedTestCases(ss) {
		if resolvePointer(tc.result) != tc.expect {
			fail(FailString("Native Object ScalarStuff: "+tc.what, tc.expect, tc.result))
		}
	}

	// test native object unpack
	fbb.Reset()
	ss = buildAssignedTable(fbb)
	ss.UnPackTo(&obj)
	expectEq := func(what string, a, b interface{}) {
		if resolvePointer(a) != b {
			fail(FailString("Native Object Unpack ScalarStuff: "+what, b, a))
		}
	}
	expectEq("justI8", obj.JustI8, int8(5))
	expectEq("maybeI8", obj.MaybeI8, int8(5))
	expectEq("defaultI8", obj.DefaultI8, int8(5))
	expectEq("justU8", obj.JustU8, byte(6))
	expectEq("maybeU8", obj.MaybeU8, byte(6))
	expectEq("defaultU8", obj.DefaultU8, byte(6))
	expectEq("justI16", obj.JustI16, int16(7))
	expectEq("maybeI16", obj.MaybeI16, int16(7))
	expectEq("defaultI16", obj.DefaultI16, int16(7))
	expectEq("justU16", obj.JustU16, uint16(8))
	expectEq("maybeU16", obj.MaybeU16, uint16(8))
	expectEq("defaultU16", obj.DefaultU16, uint16(8))
	expectEq("justI32", obj.JustI32, int32(9))
	expectEq("maybeI32", obj.MaybeI32, int32(9))
	expectEq("defaultI32", obj.DefaultI32, int32(9))
	expectEq("justU32", obj.JustU32, uint32(10))
	expectEq("maybeU32", obj.MaybeU32, uint32(10))
	expectEq("defaultU32", obj.DefaultU32, uint32(10))
	expectEq("justI64", obj.JustI64, int64(11))
	expectEq("maybeI64", obj.MaybeI64, int64(11))
	expectEq("defaultI64", obj.DefaultI64, int64(11))
	expectEq("justU64", obj.JustU64, uint64(12))
	expectEq("maybeU64", obj.MaybeU64, uint64(12))
	expectEq("defaultU64", obj.DefaultU64, uint64(12))
	expectEq("justF32", obj.JustF32, float32(13))
	expectEq("maybeF32", obj.MaybeF32, float32(13))
	expectEq("defaultF32", obj.DefaultF32, float32(13))
	expectEq("justF64", obj.JustF64, float64(14))
	expectEq("maybeF64", obj.MaybeF64, float64(14))
	expectEq("defaultF64", obj.DefaultF64, float64(14))
	expectEq("justBool", obj.JustBool, true)
	expectEq("maybeBool", obj.MaybeBool, true)
	expectEq("defaultBool", obj.DefaultBool, false)
	expectEq("justEnum", obj.JustEnum, optional_scalars.OptionalByteTwo)
	expectEq("maybeEnum", obj.MaybeEnum, optional_scalars.OptionalByteTwo)
	expectEq("defaultEnum", obj.DefaultEnum, optional_scalars.OptionalByteTwo)
}

func CheckByKey(fail func(string, ...interface{})) {
	expectEq := func(what string, a, b interface{}) {
		if a != b {
			fail(FailString("Lookup by key: "+what, b, a))
		}
	}

	b := flatbuffers.NewBuilder(0)
	name := b.CreateString("Boss")

	slime := &example.MonsterT{Name: "Slime"}
	pig := &example.MonsterT{Name: "Pig"}
	slimeBoss := &example.MonsterT{Name: "SlimeBoss"}
	mushroom := &example.MonsterT{Name: "Mushroom"}
	ironPig := &example.MonsterT{Name: "Iron Pig"}

	monsterOffsets := make([]flatbuffers.UOffsetT, 5)
	monsterOffsets[0] = slime.Pack(b)
	monsterOffsets[1] = pig.Pack(b)
	monsterOffsets[2] = slimeBoss.Pack(b)
	monsterOffsets[3] = mushroom.Pack(b)
	monsterOffsets[4] = ironPig.Pack(b)
	testarrayoftables := b.CreateVectorOfSortedTables(monsterOffsets, example.MonsterKeyCompare)

	str := &example.StatT{Id: "Strength", Count: 42}
	luk := &example.StatT{Id: "Luck", Count: 51}
	hp := &example.StatT{Id: "Health", Count: 12}
	// Test default count value of 0
	mp := &example.StatT{Id: "Mana"}

	statOffsets := make([]flatbuffers.UOffsetT, 4)
	statOffsets[0] = str.Pack(b)
	statOffsets[1] = luk.Pack(b)
	statOffsets[2] = hp.Pack(b)
	statOffsets[3] = mp.Pack(b)
	scalarKeySortedTablesOffset := b.CreateVectorOfSortedTables(statOffsets, example.StatKeyCompare)

	example.MonsterStart(b)
	example.MonsterAddName(b, name)
	example.MonsterAddTestarrayoftables(b, testarrayoftables)
	example.MonsterAddScalarKeySortedTables(b, scalarKeySortedTablesOffset)
	moff := example.MonsterEnd(b)
	b.Finish(moff)

	monster := example.GetRootAsMonster(b.Bytes, b.Head())
	slimeMon := &example.Monster{}
	monster.TestarrayoftablesByKey(slimeMon, slime.Name)
	mushroomMon := &example.Monster{}
	monster.TestarrayoftablesByKey(mushroomMon, mushroom.Name)
	slimeBossMon := &example.Monster{}
	monster.TestarrayoftablesByKey(slimeBossMon, slimeBoss.Name)

	strStat := &example.Stat{}
	monster.ScalarKeySortedTablesByKey(strStat, str.Count)
	lukStat := &example.Stat{}
	monster.ScalarKeySortedTablesByKey(lukStat, luk.Count)
	mpStat := &example.Stat{}
	monster.ScalarKeySortedTablesByKey(mpStat, mp.Count)

	expectEq("Boss name", string(monster.Name()), "Boss")
	expectEq("Slime name", string(slimeMon.Name()), slime.Name)
	expectEq("Mushroom name", string(mushroomMon.Name()), mushroom.Name)
	expectEq("SlimeBoss name", string(slimeBossMon.Name()), slimeBoss.Name)
	expectEq("Strength Id", string(strStat.Id()), str.Id)
	expectEq("Strength Count", strStat.Count(), str.Count)
	expectEq("Luck Id", string(lukStat.Id()), luk.Id)
	expectEq("Luck Count", lukStat.Count(), luk.Count)
	expectEq("Mana Id", string(mpStat.Id()), mp.Id)
	// Use default count value as key
	expectEq("Mana Count", mpStat.Count(), uint16(0))
}

// BenchmarkVtableDeduplication measures the speed of vtable deduplication
// by creating prePop vtables, then populating b.N objects with a
// different single vtable.
//
// When b.N is large (as in long benchmarks), memory usage may be high.
func BenchmarkVtableDeduplication(b *testing.B) {
	prePop := 10
	builder := flatbuffers.NewBuilder(0)

	// pre-populate some vtables:
	for i := 0; i < prePop; i++ {
		builder.StartObject(i)
		for j := 0; j < i; j++ {
			builder.PrependInt16Slot(j, int16(j), 0)
		}
		builder.EndObject()
	}

	// benchmark deduplication of a new vtable:
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lim := prePop

		builder.StartObject(lim)
		for j := 0; j < lim; j++ {
			builder.PrependInt16Slot(j, int16(j), 0)
		}
		builder.EndObject()
	}
}

// BenchmarkParseGold measures the speed of parsing the 'gold' data
// used throughout this test suite.
func BenchmarkParseGold(b *testing.B) {
	buf, offset := CheckGeneratedBuild(false, b.Fatalf)
	monster := example.GetRootAsMonster(buf, offset)

	// use these to prevent allocations:
	reuse_pos := example.Vec3{}
	reuse_test3 := example.Test{}
	reuse_table2 := flatbuffers.Table{}
	reuse_monster2 := example.Monster{}
	reuse_test4_0 := example.Test{}
	reuse_test4_1 := example.Test{}

	b.SetBytes(int64(len(buf[offset:])))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		monster.Hp()
		monster.Mana()
		name := monster.Name()
		_ = name[0]
		_ = name[len(name)-1]

		monster.Pos(&reuse_pos)
		reuse_pos.X()
		reuse_pos.Y()
		reuse_pos.Z()
		reuse_pos.Test1()
		reuse_pos.Test2()
		reuse_pos.Test3(&reuse_test3)
		reuse_test3.A()
		reuse_test3.B()
		monster.TestType()
		monster.Test(&reuse_table2)
		reuse_monster2.Init(reuse_table2.Bytes, reuse_table2.Pos)
		name2 := reuse_monster2.Name()
		_ = name2[0]
		_ = name2[len(name2)-1]
		monster.InventoryLength()
		l := monster.InventoryLength()
		for i := 0; i < l; i++ {
			monster.Inventory(i)
		}
		monster.Test4Length()
		monster.Test4(&reuse_test4_0, 0)
		monster.Test4(&reuse_test4_1, 1)

		reuse_test4_0.A()
		reuse_test4_0.B()
		reuse_test4_1.A()
		reuse_test4_1.B()

		monster.TestarrayofstringLength()
		str0 := monster.Testarrayofstring(0)
		_ = str0[0]
		_ = str0[len(str0)-1]
		str1 := monster.Testarrayofstring(1)
		_ = str1[0]
		_ = str1[len(str1)-1]
	}
}

// BenchmarkBuildGold uses generated code to build the example Monster.
func BenchmarkBuildGold(b *testing.B) {
	buf, offset := CheckGeneratedBuild(false, b.Fatalf)
	bytes_length := int64(len(buf[offset:]))

	reuse_str := "MyMonster"
	reuse_test1 := "test1"
	reuse_test2 := "test2"
	reuse_fred := "Fred"

	b.SetBytes(bytes_length)
	bldr := flatbuffers.NewBuilder(0)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		bldr.Reset()

		str := bldr.CreateString(reuse_str)
		test1 := bldr.CreateString(reuse_test1)
		test2 := bldr.CreateString(reuse_test2)
		fred := bldr.CreateString(reuse_fred)

		example.MonsterStartInventoryVector(bldr, 5)
		bldr.PrependByte(4)
		bldr.PrependByte(3)
		bldr.PrependByte(2)
		bldr.PrependByte(1)
		bldr.PrependByte(0)
		inv := bldr.EndVector(5)

		example.MonsterStart(bldr)
		example.MonsterAddName(bldr, fred)
		mon2 := example.MonsterEnd(bldr)

		example.MonsterStartTest4Vector(bldr, 2)
		example.CreateTest(bldr, 10, 20)
		example.CreateTest(bldr, 30, 40)
		test4 := bldr.EndVector(2)

		example.MonsterStartTestarrayofstringVector(bldr, 2)
		bldr.PrependUOffsetT(test2)
		bldr.PrependUOffsetT(test1)
		testArrayOfString := bldr.EndVector(2)

		example.MonsterStart(bldr)

		pos := example.CreateVec3(bldr, 1.0, 2.0, 3.0, 3.0, example.ColorGreen, 5, 6)
		example.MonsterAddPos(bldr, pos)

		example.MonsterAddHp(bldr, 80)
		example.MonsterAddName(bldr, str)
		example.MonsterAddInventory(bldr, inv)
		example.MonsterAddTestType(bldr, 1)
		example.MonsterAddTest(bldr, mon2)
		example.MonsterAddTest4(bldr, test4)
		example.MonsterAddTestarrayofstring(bldr, testArrayOfString)
		mon := example.MonsterEnd(bldr)

		bldr.Finish(mon)
	}
}
