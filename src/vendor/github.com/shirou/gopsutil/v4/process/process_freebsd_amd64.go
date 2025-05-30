// SPDX-License-Identifier: BSD-3-Clause
// Code generated by cmd/cgo -godefs; DO NOT EDIT.
// cgo -godefs types_freebsd.go

package process

const (
	CTLKern          = 1
	KernProc         = 14
	KernProcPID      = 1
	KernProcProc     = 8
	KernProcPathname = 12
	KernProcArgs     = 7
	KernProcCwd      = 42
)

const (
	sizeofPtr      = 0x8
	sizeofShort    = 0x2
	sizeofInt      = 0x4
	sizeofLong     = 0x8
	sizeofLongLong = 0x8
)

const (
	sizeOfKinfoVmentry = 0x488
	sizeOfKinfoProc    = 0x440
	sizeOfKinfoFile    = 0x570
)

const (
	SIDL   = 1
	SRUN   = 2
	SSLEEP = 3
	SSTOP  = 4
	SZOMB  = 5
	SWAIT  = 6
	SLOCK  = 7
)

type (
	_C_short     int16
	_C_int       int32
	_C_long      int64
	_C_long_long int64
)

type Timespec struct {
	Sec  int64
	Nsec int64
}

type Timeval struct {
	Sec  int64
	Usec int64
}

type Rusage struct {
	Utime    Timeval
	Stime    Timeval
	Maxrss   int64
	Ixrss    int64
	Idrss    int64
	Isrss    int64
	Minflt   int64
	Majflt   int64
	Nswap    int64
	Inblock  int64
	Oublock  int64
	Msgsnd   int64
	Msgrcv   int64
	Nsignals int64
	Nvcsw    int64
	Nivcsw   int64
}

type Rlimit struct {
	Cur int64
	Max int64
}

type KinfoProc struct {
	Structsize     int32
	Layout         int32
	Args           int64 /* pargs */
	Paddr          int64 /* proc */
	Addr           int64 /* user */
	Tracep         int64 /* vnode */
	Textvp         int64 /* vnode */
	Fd             int64 /* filedesc */
	Vmspace        int64 /* vmspace */
	Wchan          int64
	Pid            int32
	Ppid           int32
	Pgid           int32
	Tpgid          int32
	Sid            int32
	Tsid           int32
	Jobc           int16
	Spare_short1   int16
	Tdev_freebsd11 uint32
	Siglist        [16]byte /* sigset */
	Sigmask        [16]byte /* sigset */
	Sigignore      [16]byte /* sigset */
	Sigcatch       [16]byte /* sigset */
	Uid            uint32
	Ruid           uint32
	Svuid          uint32
	Rgid           uint32
	Svgid          uint32
	Ngroups        int16
	Spare_short2   int16
	Groups         [16]uint32
	Size           uint64
	Rssize         int64
	Swrss          int64
	Tsize          int64
	Dsize          int64
	Ssize          int64
	Xstat          uint16
	Acflag         uint16
	Pctcpu         uint32
	Estcpu         uint32
	Slptime        uint32
	Swtime         uint32
	Cow            uint32
	Runtime        uint64
	Start          Timeval
	Childtime      Timeval
	Flag           int64
	Kiflag         int64
	Traceflag      int32
	Stat           int8
	Nice           int8
	Lock           int8
	Rqindex        int8
	Oncpu_old      uint8
	Lastcpu_old    uint8
	Tdname         [17]int8
	Wmesg          [9]int8
	Login          [18]int8
	Lockname       [9]int8
	Comm           [20]int8
	Emul           [17]int8
	Loginclass     [18]int8
	Moretdname     [4]int8
	Sparestrings   [46]int8
	Spareints      [2]int32
	Tdev           uint64
	Oncpu          int32
	Lastcpu        int32
	Tracer         int32
	Flag2          int32
	Fibnum         int32
	Cr_flags       uint32
	Jid            int32
	Numthreads     int32
	Tid            int32
	Pri            Priority
	Rusage         Rusage
	Rusage_ch      Rusage
	Pcb            int64 /* pcb */
	Kstack         int64
	Udata          int64
	Tdaddr         int64 /* thread */
	Pd             int64 /* pwddesc, not accurate */
	Spareptrs      [5]int64
	Sparelongs     [12]int64
	Sflag          int64
	Tdflags        int64
}

type Priority struct {
	Class  uint8
	Level  uint8
	Native uint8
	User   uint8
}

type KinfoVmentry struct {
	Structsize        int32
	Type              int32
	Start             uint64
	End               uint64
	Offset            uint64
	Vn_fileid         uint64
	Vn_fsid_freebsd11 uint32
	Flags             int32
	Resident          int32
	Private_resident  int32
	Protection        int32
	Ref_count         int32
	Shadow_count      int32
	Vn_type           int32
	Vn_size           uint64
	Vn_rdev_freebsd11 uint32
	Vn_mode           uint16
	Status            uint16
	Type_spec         [8]byte
	Vn_rdev           uint64
	X_kve_ispare      [8]int32
	Path              [1024]int8
}

type kinfoFile struct {
	Structsize     int32
	Type           int32
	Fd             int32
	Ref_count      int32
	Flags          int32
	Pad0           int32
	Offset         int64
	Anon0          [304]byte
	Status         uint16
	Pad1           uint16
	X_kf_ispare0   int32
	Cap_rights     capRights
	X_kf_cap_spare uint64
	Path           [1024]int8
}

type capRights struct {
	Rights [2]uint64
}
