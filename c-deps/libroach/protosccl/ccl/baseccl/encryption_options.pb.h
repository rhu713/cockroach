// Generated by the protocol buffer compiler.  DO NOT EDIT!
// source: ccl/baseccl/encryption_options.proto

#ifndef PROTOBUF_ccl_2fbaseccl_2fencryption_5foptions_2eproto__INCLUDED
#define PROTOBUF_ccl_2fbaseccl_2fencryption_5foptions_2eproto__INCLUDED

#include <string>

#include <google/protobuf/stubs/common.h>

#if GOOGLE_PROTOBUF_VERSION < 3004000
#error This file was generated by a newer version of protoc which is
#error incompatible with your Protocol Buffer headers.  Please update
#error your headers.
#endif
#if 3004000 < GOOGLE_PROTOBUF_MIN_PROTOC_VERSION
#error This file was generated by an older version of protoc which is
#error incompatible with your Protocol Buffer headers.  Please
#error regenerate this file with a newer version of protoc.
#endif

#include <google/protobuf/io/coded_stream.h>
#include <google/protobuf/arena.h>
#include <google/protobuf/arenastring.h>
#include <google/protobuf/generated_message_table_driven.h>
#include <google/protobuf/generated_message_util.h>
#include <google/protobuf/metadata_lite.h>
#include <google/protobuf/message_lite.h>
#include <google/protobuf/repeated_field.h>  // IWYU pragma: export
#include <google/protobuf/extension_set.h>  // IWYU pragma: export
#include <google/protobuf/generated_enum_util.h>
// @@protoc_insertion_point(includes)
namespace cockroach {
namespace ccl {
namespace baseccl {
class EncryptionKeyFiles;
class EncryptionKeyFilesDefaultTypeInternal;
extern EncryptionKeyFilesDefaultTypeInternal _EncryptionKeyFiles_default_instance_;
class EncryptionOptions;
class EncryptionOptionsDefaultTypeInternal;
extern EncryptionOptionsDefaultTypeInternal _EncryptionOptions_default_instance_;
}  // namespace baseccl
}  // namespace ccl
}  // namespace cockroach

namespace cockroach {
namespace ccl {
namespace baseccl {

namespace protobuf_ccl_2fbaseccl_2fencryption_5foptions_2eproto {
// Internal implementation detail -- do not call these.
struct TableStruct {
  static const ::google::protobuf::internal::ParseTableField entries[];
  static const ::google::protobuf::internal::AuxillaryParseTableField aux[];
  static const ::google::protobuf::internal::ParseTable schema[];
  static const ::google::protobuf::uint32 offsets[];
  static const ::google::protobuf::internal::FieldMetadata field_metadata[];
  static const ::google::protobuf::internal::SerializationTable serialization_table[];
  static void InitDefaultsImpl();
};
void AddDescriptors();
void InitDefaults();
}  // namespace protobuf_ccl_2fbaseccl_2fencryption_5foptions_2eproto

enum EncryptionKeySource {
  KeyFiles = 0,
  EncryptionKeySource_INT_MIN_SENTINEL_DO_NOT_USE_ = ::google::protobuf::kint32min,
  EncryptionKeySource_INT_MAX_SENTINEL_DO_NOT_USE_ = ::google::protobuf::kint32max
};
bool EncryptionKeySource_IsValid(int value);
const EncryptionKeySource EncryptionKeySource_MIN = KeyFiles;
const EncryptionKeySource EncryptionKeySource_MAX = KeyFiles;
const int EncryptionKeySource_ARRAYSIZE = EncryptionKeySource_MAX + 1;

// ===================================================================

class EncryptionKeyFiles : public ::google::protobuf::MessageLite /* @@protoc_insertion_point(class_definition:cockroach.ccl.baseccl.EncryptionKeyFiles) */ {
 public:
  EncryptionKeyFiles();
  virtual ~EncryptionKeyFiles();

  EncryptionKeyFiles(const EncryptionKeyFiles& from);

  inline EncryptionKeyFiles& operator=(const EncryptionKeyFiles& from) {
    CopyFrom(from);
    return *this;
  }
  #if LANG_CXX11
  EncryptionKeyFiles(EncryptionKeyFiles&& from) noexcept
    : EncryptionKeyFiles() {
    *this = ::std::move(from);
  }

  inline EncryptionKeyFiles& operator=(EncryptionKeyFiles&& from) noexcept {
    if (GetArenaNoVirtual() == from.GetArenaNoVirtual()) {
      if (this != &from) InternalSwap(&from);
    } else {
      CopyFrom(from);
    }
    return *this;
  }
  #endif
  static const EncryptionKeyFiles& default_instance();

  static inline const EncryptionKeyFiles* internal_default_instance() {
    return reinterpret_cast<const EncryptionKeyFiles*>(
               &_EncryptionKeyFiles_default_instance_);
  }
  static PROTOBUF_CONSTEXPR int const kIndexInFileMessages =
    0;

  void Swap(EncryptionKeyFiles* other);
  friend void swap(EncryptionKeyFiles& a, EncryptionKeyFiles& b) {
    a.Swap(&b);
  }

  // implements Message ----------------------------------------------

  inline EncryptionKeyFiles* New() const PROTOBUF_FINAL { return New(NULL); }

  EncryptionKeyFiles* New(::google::protobuf::Arena* arena) const PROTOBUF_FINAL;
  void CheckTypeAndMergeFrom(const ::google::protobuf::MessageLite& from)
    PROTOBUF_FINAL;
  void CopyFrom(const EncryptionKeyFiles& from);
  void MergeFrom(const EncryptionKeyFiles& from);
  void Clear() PROTOBUF_FINAL;
  bool IsInitialized() const PROTOBUF_FINAL;

  size_t ByteSizeLong() const PROTOBUF_FINAL;
  bool MergePartialFromCodedStream(
      ::google::protobuf::io::CodedInputStream* input) PROTOBUF_FINAL;
  void SerializeWithCachedSizes(
      ::google::protobuf::io::CodedOutputStream* output) const PROTOBUF_FINAL;
  void DiscardUnknownFields();
  int GetCachedSize() const PROTOBUF_FINAL { return _cached_size_; }
  private:
  void SharedCtor();
  void SharedDtor();
  void SetCachedSize(int size) const;
  void InternalSwap(EncryptionKeyFiles* other);
  private:
  inline ::google::protobuf::Arena* GetArenaNoVirtual() const {
    return NULL;
  }
  inline void* MaybeArenaPtr() const {
    return NULL;
  }
  public:

  ::std::string GetTypeName() const PROTOBUF_FINAL;

  // nested types ----------------------------------------------------

  // accessors -------------------------------------------------------

  // string current_key = 1;
  void clear_current_key();
  static const int kCurrentKeyFieldNumber = 1;
  const ::std::string& current_key() const;
  void set_current_key(const ::std::string& value);
  #if LANG_CXX11
  void set_current_key(::std::string&& value);
  #endif
  void set_current_key(const char* value);
  void set_current_key(const char* value, size_t size);
  ::std::string* mutable_current_key();
  ::std::string* release_current_key();
  void set_allocated_current_key(::std::string* current_key);

  // string old_key = 2;
  void clear_old_key();
  static const int kOldKeyFieldNumber = 2;
  const ::std::string& old_key() const;
  void set_old_key(const ::std::string& value);
  #if LANG_CXX11
  void set_old_key(::std::string&& value);
  #endif
  void set_old_key(const char* value);
  void set_old_key(const char* value, size_t size);
  ::std::string* mutable_old_key();
  ::std::string* release_old_key();
  void set_allocated_old_key(::std::string* old_key);

  // @@protoc_insertion_point(class_scope:cockroach.ccl.baseccl.EncryptionKeyFiles)
 private:

  ::google::protobuf::internal::InternalMetadataWithArenaLite _internal_metadata_;
  ::google::protobuf::internal::ArenaStringPtr current_key_;
  ::google::protobuf::internal::ArenaStringPtr old_key_;
  mutable int _cached_size_;
  friend struct protobuf_ccl_2fbaseccl_2fencryption_5foptions_2eproto::TableStruct;
};
// -------------------------------------------------------------------

class EncryptionOptions : public ::google::protobuf::MessageLite /* @@protoc_insertion_point(class_definition:cockroach.ccl.baseccl.EncryptionOptions) */ {
 public:
  EncryptionOptions();
  virtual ~EncryptionOptions();

  EncryptionOptions(const EncryptionOptions& from);

  inline EncryptionOptions& operator=(const EncryptionOptions& from) {
    CopyFrom(from);
    return *this;
  }
  #if LANG_CXX11
  EncryptionOptions(EncryptionOptions&& from) noexcept
    : EncryptionOptions() {
    *this = ::std::move(from);
  }

  inline EncryptionOptions& operator=(EncryptionOptions&& from) noexcept {
    if (GetArenaNoVirtual() == from.GetArenaNoVirtual()) {
      if (this != &from) InternalSwap(&from);
    } else {
      CopyFrom(from);
    }
    return *this;
  }
  #endif
  static const EncryptionOptions& default_instance();

  static inline const EncryptionOptions* internal_default_instance() {
    return reinterpret_cast<const EncryptionOptions*>(
               &_EncryptionOptions_default_instance_);
  }
  static PROTOBUF_CONSTEXPR int const kIndexInFileMessages =
    1;

  void Swap(EncryptionOptions* other);
  friend void swap(EncryptionOptions& a, EncryptionOptions& b) {
    a.Swap(&b);
  }

  // implements Message ----------------------------------------------

  inline EncryptionOptions* New() const PROTOBUF_FINAL { return New(NULL); }

  EncryptionOptions* New(::google::protobuf::Arena* arena) const PROTOBUF_FINAL;
  void CheckTypeAndMergeFrom(const ::google::protobuf::MessageLite& from)
    PROTOBUF_FINAL;
  void CopyFrom(const EncryptionOptions& from);
  void MergeFrom(const EncryptionOptions& from);
  void Clear() PROTOBUF_FINAL;
  bool IsInitialized() const PROTOBUF_FINAL;

  size_t ByteSizeLong() const PROTOBUF_FINAL;
  bool MergePartialFromCodedStream(
      ::google::protobuf::io::CodedInputStream* input) PROTOBUF_FINAL;
  void SerializeWithCachedSizes(
      ::google::protobuf::io::CodedOutputStream* output) const PROTOBUF_FINAL;
  void DiscardUnknownFields();
  int GetCachedSize() const PROTOBUF_FINAL { return _cached_size_; }
  private:
  void SharedCtor();
  void SharedDtor();
  void SetCachedSize(int size) const;
  void InternalSwap(EncryptionOptions* other);
  private:
  inline ::google::protobuf::Arena* GetArenaNoVirtual() const {
    return NULL;
  }
  inline void* MaybeArenaPtr() const {
    return NULL;
  }
  public:

  ::std::string GetTypeName() const PROTOBUF_FINAL;

  // nested types ----------------------------------------------------

  // accessors -------------------------------------------------------

  // .cockroach.ccl.baseccl.EncryptionKeyFiles key_files = 2;
  bool has_key_files() const;
  void clear_key_files();
  static const int kKeyFilesFieldNumber = 2;
  const ::cockroach::ccl::baseccl::EncryptionKeyFiles& key_files() const;
  ::cockroach::ccl::baseccl::EncryptionKeyFiles* mutable_key_files();
  ::cockroach::ccl::baseccl::EncryptionKeyFiles* release_key_files();
  void set_allocated_key_files(::cockroach::ccl::baseccl::EncryptionKeyFiles* key_files);

  // int64 data_key_rotation_period = 3;
  void clear_data_key_rotation_period();
  static const int kDataKeyRotationPeriodFieldNumber = 3;
  ::google::protobuf::int64 data_key_rotation_period() const;
  void set_data_key_rotation_period(::google::protobuf::int64 value);

  // .cockroach.ccl.baseccl.EncryptionKeySource key_source = 1;
  void clear_key_source();
  static const int kKeySourceFieldNumber = 1;
  ::cockroach::ccl::baseccl::EncryptionKeySource key_source() const;
  void set_key_source(::cockroach::ccl::baseccl::EncryptionKeySource value);

  // @@protoc_insertion_point(class_scope:cockroach.ccl.baseccl.EncryptionOptions)
 private:

  ::google::protobuf::internal::InternalMetadataWithArenaLite _internal_metadata_;
  ::cockroach::ccl::baseccl::EncryptionKeyFiles* key_files_;
  ::google::protobuf::int64 data_key_rotation_period_;
  int key_source_;
  mutable int _cached_size_;
  friend struct protobuf_ccl_2fbaseccl_2fencryption_5foptions_2eproto::TableStruct;
};
// ===================================================================


// ===================================================================

#if !PROTOBUF_INLINE_NOT_IN_HEADERS
#ifdef __GNUC__
  #pragma GCC diagnostic push
  #pragma GCC diagnostic ignored "-Wstrict-aliasing"
#endif  // __GNUC__
// EncryptionKeyFiles

// string current_key = 1;
inline void EncryptionKeyFiles::clear_current_key() {
  current_key_.ClearToEmptyNoArena(&::google::protobuf::internal::GetEmptyStringAlreadyInited());
}
inline const ::std::string& EncryptionKeyFiles::current_key() const {
  // @@protoc_insertion_point(field_get:cockroach.ccl.baseccl.EncryptionKeyFiles.current_key)
  return current_key_.GetNoArena();
}
inline void EncryptionKeyFiles::set_current_key(const ::std::string& value) {
  
  current_key_.SetNoArena(&::google::protobuf::internal::GetEmptyStringAlreadyInited(), value);
  // @@protoc_insertion_point(field_set:cockroach.ccl.baseccl.EncryptionKeyFiles.current_key)
}
#if LANG_CXX11
inline void EncryptionKeyFiles::set_current_key(::std::string&& value) {
  
  current_key_.SetNoArena(
    &::google::protobuf::internal::GetEmptyStringAlreadyInited(), ::std::move(value));
  // @@protoc_insertion_point(field_set_rvalue:cockroach.ccl.baseccl.EncryptionKeyFiles.current_key)
}
#endif
inline void EncryptionKeyFiles::set_current_key(const char* value) {
  GOOGLE_DCHECK(value != NULL);
  
  current_key_.SetNoArena(&::google::protobuf::internal::GetEmptyStringAlreadyInited(), ::std::string(value));
  // @@protoc_insertion_point(field_set_char:cockroach.ccl.baseccl.EncryptionKeyFiles.current_key)
}
inline void EncryptionKeyFiles::set_current_key(const char* value, size_t size) {
  
  current_key_.SetNoArena(&::google::protobuf::internal::GetEmptyStringAlreadyInited(),
      ::std::string(reinterpret_cast<const char*>(value), size));
  // @@protoc_insertion_point(field_set_pointer:cockroach.ccl.baseccl.EncryptionKeyFiles.current_key)
}
inline ::std::string* EncryptionKeyFiles::mutable_current_key() {
  
  // @@protoc_insertion_point(field_mutable:cockroach.ccl.baseccl.EncryptionKeyFiles.current_key)
  return current_key_.MutableNoArena(&::google::protobuf::internal::GetEmptyStringAlreadyInited());
}
inline ::std::string* EncryptionKeyFiles::release_current_key() {
  // @@protoc_insertion_point(field_release:cockroach.ccl.baseccl.EncryptionKeyFiles.current_key)
  
  return current_key_.ReleaseNoArena(&::google::protobuf::internal::GetEmptyStringAlreadyInited());
}
inline void EncryptionKeyFiles::set_allocated_current_key(::std::string* current_key) {
  if (current_key != NULL) {
    
  } else {
    
  }
  current_key_.SetAllocatedNoArena(&::google::protobuf::internal::GetEmptyStringAlreadyInited(), current_key);
  // @@protoc_insertion_point(field_set_allocated:cockroach.ccl.baseccl.EncryptionKeyFiles.current_key)
}

// string old_key = 2;
inline void EncryptionKeyFiles::clear_old_key() {
  old_key_.ClearToEmptyNoArena(&::google::protobuf::internal::GetEmptyStringAlreadyInited());
}
inline const ::std::string& EncryptionKeyFiles::old_key() const {
  // @@protoc_insertion_point(field_get:cockroach.ccl.baseccl.EncryptionKeyFiles.old_key)
  return old_key_.GetNoArena();
}
inline void EncryptionKeyFiles::set_old_key(const ::std::string& value) {
  
  old_key_.SetNoArena(&::google::protobuf::internal::GetEmptyStringAlreadyInited(), value);
  // @@protoc_insertion_point(field_set:cockroach.ccl.baseccl.EncryptionKeyFiles.old_key)
}
#if LANG_CXX11
inline void EncryptionKeyFiles::set_old_key(::std::string&& value) {
  
  old_key_.SetNoArena(
    &::google::protobuf::internal::GetEmptyStringAlreadyInited(), ::std::move(value));
  // @@protoc_insertion_point(field_set_rvalue:cockroach.ccl.baseccl.EncryptionKeyFiles.old_key)
}
#endif
inline void EncryptionKeyFiles::set_old_key(const char* value) {
  GOOGLE_DCHECK(value != NULL);
  
  old_key_.SetNoArena(&::google::protobuf::internal::GetEmptyStringAlreadyInited(), ::std::string(value));
  // @@protoc_insertion_point(field_set_char:cockroach.ccl.baseccl.EncryptionKeyFiles.old_key)
}
inline void EncryptionKeyFiles::set_old_key(const char* value, size_t size) {
  
  old_key_.SetNoArena(&::google::protobuf::internal::GetEmptyStringAlreadyInited(),
      ::std::string(reinterpret_cast<const char*>(value), size));
  // @@protoc_insertion_point(field_set_pointer:cockroach.ccl.baseccl.EncryptionKeyFiles.old_key)
}
inline ::std::string* EncryptionKeyFiles::mutable_old_key() {
  
  // @@protoc_insertion_point(field_mutable:cockroach.ccl.baseccl.EncryptionKeyFiles.old_key)
  return old_key_.MutableNoArena(&::google::protobuf::internal::GetEmptyStringAlreadyInited());
}
inline ::std::string* EncryptionKeyFiles::release_old_key() {
  // @@protoc_insertion_point(field_release:cockroach.ccl.baseccl.EncryptionKeyFiles.old_key)
  
  return old_key_.ReleaseNoArena(&::google::protobuf::internal::GetEmptyStringAlreadyInited());
}
inline void EncryptionKeyFiles::set_allocated_old_key(::std::string* old_key) {
  if (old_key != NULL) {
    
  } else {
    
  }
  old_key_.SetAllocatedNoArena(&::google::protobuf::internal::GetEmptyStringAlreadyInited(), old_key);
  // @@protoc_insertion_point(field_set_allocated:cockroach.ccl.baseccl.EncryptionKeyFiles.old_key)
}

// -------------------------------------------------------------------

// EncryptionOptions

// .cockroach.ccl.baseccl.EncryptionKeySource key_source = 1;
inline void EncryptionOptions::clear_key_source() {
  key_source_ = 0;
}
inline ::cockroach::ccl::baseccl::EncryptionKeySource EncryptionOptions::key_source() const {
  // @@protoc_insertion_point(field_get:cockroach.ccl.baseccl.EncryptionOptions.key_source)
  return static_cast< ::cockroach::ccl::baseccl::EncryptionKeySource >(key_source_);
}
inline void EncryptionOptions::set_key_source(::cockroach::ccl::baseccl::EncryptionKeySource value) {
  
  key_source_ = value;
  // @@protoc_insertion_point(field_set:cockroach.ccl.baseccl.EncryptionOptions.key_source)
}

// .cockroach.ccl.baseccl.EncryptionKeyFiles key_files = 2;
inline bool EncryptionOptions::has_key_files() const {
  return this != internal_default_instance() && key_files_ != NULL;
}
inline void EncryptionOptions::clear_key_files() {
  if (GetArenaNoVirtual() == NULL && key_files_ != NULL) delete key_files_;
  key_files_ = NULL;
}
inline const ::cockroach::ccl::baseccl::EncryptionKeyFiles& EncryptionOptions::key_files() const {
  const ::cockroach::ccl::baseccl::EncryptionKeyFiles* p = key_files_;
  // @@protoc_insertion_point(field_get:cockroach.ccl.baseccl.EncryptionOptions.key_files)
  return p != NULL ? *p : *reinterpret_cast<const ::cockroach::ccl::baseccl::EncryptionKeyFiles*>(
      &::cockroach::ccl::baseccl::_EncryptionKeyFiles_default_instance_);
}
inline ::cockroach::ccl::baseccl::EncryptionKeyFiles* EncryptionOptions::mutable_key_files() {
  
  if (key_files_ == NULL) {
    key_files_ = new ::cockroach::ccl::baseccl::EncryptionKeyFiles;
  }
  // @@protoc_insertion_point(field_mutable:cockroach.ccl.baseccl.EncryptionOptions.key_files)
  return key_files_;
}
inline ::cockroach::ccl::baseccl::EncryptionKeyFiles* EncryptionOptions::release_key_files() {
  // @@protoc_insertion_point(field_release:cockroach.ccl.baseccl.EncryptionOptions.key_files)
  
  ::cockroach::ccl::baseccl::EncryptionKeyFiles* temp = key_files_;
  key_files_ = NULL;
  return temp;
}
inline void EncryptionOptions::set_allocated_key_files(::cockroach::ccl::baseccl::EncryptionKeyFiles* key_files) {
  delete key_files_;
  key_files_ = key_files;
  if (key_files) {
    
  } else {
    
  }
  // @@protoc_insertion_point(field_set_allocated:cockroach.ccl.baseccl.EncryptionOptions.key_files)
}

// int64 data_key_rotation_period = 3;
inline void EncryptionOptions::clear_data_key_rotation_period() {
  data_key_rotation_period_ = GOOGLE_LONGLONG(0);
}
inline ::google::protobuf::int64 EncryptionOptions::data_key_rotation_period() const {
  // @@protoc_insertion_point(field_get:cockroach.ccl.baseccl.EncryptionOptions.data_key_rotation_period)
  return data_key_rotation_period_;
}
inline void EncryptionOptions::set_data_key_rotation_period(::google::protobuf::int64 value) {
  
  data_key_rotation_period_ = value;
  // @@protoc_insertion_point(field_set:cockroach.ccl.baseccl.EncryptionOptions.data_key_rotation_period)
}

#ifdef __GNUC__
  #pragma GCC diagnostic pop
#endif  // __GNUC__
#endif  // !PROTOBUF_INLINE_NOT_IN_HEADERS
// -------------------------------------------------------------------


// @@protoc_insertion_point(namespace_scope)


}  // namespace baseccl
}  // namespace ccl
}  // namespace cockroach

namespace google {
namespace protobuf {

template <> struct is_proto_enum< ::cockroach::ccl::baseccl::EncryptionKeySource> : ::google::protobuf::internal::true_type {};

}  // namespace protobuf
}  // namespace google

// @@protoc_insertion_point(global_scope)

#endif  // PROTOBUF_ccl_2fbaseccl_2fencryption_5foptions_2eproto__INCLUDED
