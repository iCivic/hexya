// Copyright 2016 NDP Systèmes. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package models

import (
	"fmt"
)

// A RecordRef uniquely identifies a Record by giving its model and ID.
type RecordRef struct {
	ModelName string
	ID        int64
}

// RecordSet identifies a type that holds a set of records of
// a given model.
type RecordSet interface {
	// ModelName returns the name of the model of this RecordSet
	ModelName() string
	// Ids returns the ids in this set of Records
	Ids() []int64
	// Env returns the current Environment of this RecordSet
	Env() Environment
	// Len returns the number of records in this RecordSet
	Len() int
	// IsEmpty returns true if this RecordSet has no records
	IsEmpty() bool
	// IsNotEmpty returns true if this RecordSet has at least one record
	IsNotEmpty() bool
	// Call executes the given method (as string) with the given arguments
	Call(string, ...interface{}) interface{}
	// Collection returns the underlying RecordCollection instance
	Collection() *RecordCollection
	// Get returns the value of the given fieldName for the first record of this RecordCollection.
	// It returns the type's zero value if the RecordCollection is empty.
	Get(string) interface{}
	// Set sets field given by fieldName to the given value. If the RecordSet has several
	// Records, all of them will be updated. Each call to Set makes an update query in the
	// database. It panics if it is called on an empty RecordSet.
	Set(string, interface{})
	// T translates the given string to the language specified by
	// the 'lang' key of rc.Env().Context(). If for any reason the
	// string cannot be translated, then src is returned.
	//
	// You MUST pass a string literal as src to have it extracted automatically (and not a variable)
	//
	// The translated string will be passed to fmt.Sprintf with the optional args
	// before being returned.
	T(string, ...interface{}) string
	// EnsureOne panics if this Recordset is not a singleton
	EnsureOne()
}

// A FieldName is a type representing field names in models.
type FieldName string

// FieldName makes a FieldName instance a FieldNamer
func (fn FieldName) FieldName() FieldName {
	return fn
}

// String function for FieldName
func (fn FieldName) String() string {
	return string(fn)
}

var _ FieldNamer = FieldName("")

// A FieldNamer is a type that can yield a FieldName through
// its FieldName() method
type FieldNamer interface {
	fmt.Stringer
	FieldName() FieldName
}

// A GroupAggregateRow holds a row of results of a query with a group by clause
// - Values holds the values of the actual query
// - Count is the number of lines aggregated into this one
// - Condition can be used to query the aggregated rows separately if needed
type GroupAggregateRow struct {
	Values    *ModelData
	Count     int
	Condition *Condition
}

// FieldContexts define the different contexts for a field, that will define different
// values for this field.
//
// The key is a context name and the value is a function that returns the context
// value for the given recordset.
type FieldContexts map[string]func(RecordSet) string

// A FieldMapper is an object that can convert itself into a FieldMap
type FieldMapper interface {
	// Underlying returns the object converted to a FieldMap.
	Underlying() FieldMap
}

// A Methoder can return a Method data object through its Underlying() method
type Methoder interface {
	Underlying() *Method
}

// A Modeler can return a Model data object through its Underlying() method
type Modeler interface {
	Underlying() *Model
}

// A Conditioner can return a Condition object through its Underlying() method
type Conditioner interface {
	Underlying() *Condition
}

// A RecordData can return a ModelData object through its Underlying() method
type RecordData interface {
	Underlying() *ModelData
}

// A ModelData is used to hold values of an object instance for creating or
// updating a RecordSet. It is mainly designed to be embedded in a type-safe
// struct.
type ModelData struct {
	FieldMap
	ToCreate map[string][]*ModelData
	Model    *Model
}

var _ RecordData = new(ModelData)

// Get returns the value of the given field.
// The second returned value is true if the value exists.
//
// The field can be either its name or is JSON name.
func (md *ModelData) Get(field string) (interface{}, bool) {
	return md.FieldMap.Get(field, md.Model)
}

// Set sets the given field with the given value.
// If the field already exists, then it is updated with value.
// Otherwise, a new entry is inserted.
//
// It returns the given ModelData so that calls can be chained
func (md *ModelData) Set(field string, value interface{}) *ModelData {
	md.FieldMap.Set(field, value, md.Model)
	return md
}

// Unset removes the value of the given field if it exists.
//
// It returns the given ModelData so that calls can be chained
func (md *ModelData) Unset(field string) *ModelData {
	md.FieldMap.Delete(field, md.Model)
	return md
}

// Create stores the related ModelData to be used to create
// a related record on the fly and link it to this field.
//
// This method can be called multiple times to create multiple records
func (md *ModelData) Create(field string, related *ModelData) *ModelData {
	fi := md.Model.getRelatedFieldInfo(field)
	if related.Model != fi.relatedModel {
		log.Panic("create data must be of the model of the relation field", "fieldModel", fi.relatedModel, "dataModel", related.Model)
	}
	md.ToCreate[fi.json] = append(md.ToCreate[fi.json], related)
	return md
}

// Copy returns a copy of this ModelData
func (md *ModelData) Copy() *ModelData {
	ntc := make(map[string][]*ModelData)
	for k, v := range md.ToCreate {
		ntc[k] = v
	}
	return &ModelData{
		Model:    md.Model,
		FieldMap: md.FieldMap.Copy(),
		ToCreate: ntc,
	}
}

// Underlying returns the ModelData
func (md *ModelData) Underlying() *ModelData {
	return md
}

// NewModelData returns a pointer to a new instance of ModelData
// for the given model. If FieldMaps are given they are added to
// the ModelData.
func NewModelData(model Modeler, fm ...FieldMap) *ModelData {
	fMap := make(FieldMap)
	for _, f := range fm {
		for k, v := range f {
			fMap[model.Underlying().JSONizeFieldName(k)] = v
		}
	}
	return &ModelData{
		FieldMap: fMap,
		ToCreate: make(map[string][]*ModelData),
		Model:    model.Underlying(),
	}
}

// NewModelDataFromRS creates a pointer to a new instance of ModelData.
// If FieldMaps are given they are added to the ModelData.
//
// Unlike NewModelData, this method translates relation fields in64 and
// []int64 values as RecordSets
func NewModelDataFromRS(rs RecordSet, fm ...FieldMap) *ModelData {
	fMap := make(FieldMap)
	for _, f := range fm {
		for k, v := range f {
			fi := rs.Collection().Model().getRelatedFieldInfo(k)
			if fi.isRelationField() {
				v = rs.Collection().convertToRecordSet(v, fi.relatedModelName)
			}
			fMap[fi.json] = v
		}
	}
	return &ModelData{
		FieldMap: fMap,
		ToCreate: make(map[string][]*ModelData),
		Model:    rs.Collection().model,
	}
}
