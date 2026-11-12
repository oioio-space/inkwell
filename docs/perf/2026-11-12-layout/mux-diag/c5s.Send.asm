TEXT  
mux.go:219   LEAQ -0x68(SP), R12
mux.go:219   CMPQ R12, 0x10(R14)
mux.go:219   JBE 
mux.go:219   PUSHQ BP
mux.go:219   MOVQ SP, BP
mux.go:219   SUBQ $0xe0, SP
mux.go:219   MOVQ BX, 0xf8(SP)
mux.go:219   MOVQ CX, 0x100(SP)
mux.go:219   MOVQ DI, 0x108(SP)
mux.go:219   MOVQ R8, 0x118(SP)
mux.go:219   MOVUPS X15, 0xd0(SP)
mux.go:219   MOVB $0x0, 0x47(SP)
mux.go:219   MOVUPS X15, 0x70(SP)
mux.go:220   CMPQ R9, $
mux.go:220   JG 
mux.go:223   LEAQ 0x1(R9), DX
mux.go:223   NOPL
mux.go:223   CMPQ DX, $
mux.go:223   JG 
mux.go:220   MOVQ AX, 0xf0(SP)
mux.go:223   MOVQ DX, 0x60(SP)
mux.go:220   MOVQ R8, 0x118(SP)
mux.go:220   MOVQ SI, 0x110(SP)
mux.go:220   MOVQ DI, 0x108(SP)
mux.go:220   MOVQ CX, 0x100(SP)
mux.go:220   MOVQ BX, 0xf8(SP)
mux.go:220   MOVQ R9, 0x120(SP)
mux.go:228   MOVQ 0(AX), AX
mux.go:228   MOVQ DI, BX
mux.go:228   MOVQ SI, CX
mux.go:228   CALL github.com/oioio-space/inkwell/internal/transport/mux.(*muxer).lease(SB)
mux.go:228   TESTQ AX, AX
mux.go:228   JNE 
mux.go:228   XORL AX, AX
mux.go:228   NOPL
mux.go:228   JMP 
mux.go:228   MOVQ AX, 0x80(SP)
mux.go:229   MOVQ 0xf0(SP), DX
mux.go:229   MOVQ 0(DX), DX
mux.go:229   LEAQ github.com/oioio-space/inkwell/internal/transport/mux.(*sub).Send.deferwrap1(SB), SI
mux.go:229   MOVQ SI, 0xa8(SP)
mux.go:229   MOVQ DX, 0xb0(SP)
mux.go:229   MOVQ 0x110(SP), DX
mux.go:229   MOVQ DX, 0xc0(SP)
mux.go:229   MOVQ 0x108(SP), DX
mux.go:229   MOVQ DX, 0xb8(SP)
mux.go:229   MOVQ AX, 0xc8(SP)
mux.go:229   LEAQ 0xa8(SP), DX
mux.go:229   MOVQ DX, 0xd0(SP)
mux.go:229   MOVB $0x1, 0x47(SP)
mux.go:230   MOVQ 0xf8(SP), BX
mux.go:230   MOVQ 0x100(SP), CX
mux.go:230   XORL DI, DI
mux.go:230   NOPW
mux.go:230   CALL github.com/oioio-space/inkwell/internal/transport/mux.(*gate).acquire(SB)
mux.go:230   TESTQ AX, AX
mux.go:230   JNE 
mux.go:233   LEAQ github.com/oioio-space/inkwell/internal/transport/mux.(*sub).Send.deferwrap2(SB), CX
mux.go:233   MOVQ CX, 0x98(SP)
mux.go:233   MOVQ 0x80(SP), CX
mux.go:233   MOVQ CX, 0xa0(SP)
mux.go:233   LEAQ 0x98(SP), CX
mux.go:233   MOVQ CX, 0xd8(SP)
mux.go:233   MOVB $0x3, 0x47(SP)
mux.go:233   MOVL $0x3, AX
mux.go:236   MOVB AL, 0x46(SP)
mux.go:236   MOVQ 0xf0(SP), CX
mux.go:236   MOVQ 0(CX), AX
mux.go:245   TESTB AL, 0(AX)
mux.go:245   ADDQ $0x78, AX
mux.go:245   NOPL 0(AX)
mux.go:245   CALL sync.(*Pool).Get(SB)
mux.go:245   LEAQ (IP), CX
mux.go:245   CMPQ AX, CX
mux.go:245   JE 
mux.go:245   XORL BX, BX
mux.go:246   TESTQ BX, BX
mux.go:246   JNE 
mux.go:247   MOVL $0x18, AX
mux.go:247   LEAQ (IP), BX
mux.go:247   MOVL $0x1, CX
mux.go:247   CALL runtime.mallocgcSmallScanNoHeaderSC3(SB)
mux.go:236   MOVQ AX, BX
mux.go:236   MOVQ BX, 0x90(SP)
mux.go:237   MOVQ 0x10(BX), CX
mux.go:237   MOVQ 0(BX), AX
mux.go:254   MOVQ 0xf0(SP), DX
mux.go:254   MOVZX 0x10(DX), R8
mux.go:254   TESTQ CX, CX
mux.go:254   JNE 
mux.go:254   MOVB R8, 0x45(SP)
mux.go:254   MOVL $0x1, BX
mux.go:254   XORL CX, CX
mux.go:254   MOVL BX, DI
mux.go:254   LEAQ (IP), SI
mux.go:254   CALL runtime.growslice(SB)
mux.go:238   MOVQ 0xf0(SP), DX
mux.go:237   MOVQ 0x90(SP), BX
mux.go:254   MOVZX 0x45(SP), R8
mux.go:254   MOVB R8, 0(AX)
mux.go:254   MOVQ 0x60(SP), R8
mux.go:254   CMPQ CX, R8
mux.go:254   JAE 
mux.go:254   MOVQ R8, BX
mux.go:254   MOVQ 0x120(SP), DI
mux.go:254   LEAQ (IP), SI
mux.go:254   CALL runtime.growslice(SB)
mux.go:254   MOVQ BX, R8
mux.go:254   MOVQ CX, 0x50(SP)
mux.go:254   MOVQ R8, 0x48(SP)
mux.go:254   MOVQ AX, 0x68(SP)
mux.go:254   INCQ AX
mux.go:254   MOVQ 0x118(SP), BX
mux.go:254   MOVQ 0x120(SP), CX
mux.go:254   CALL runtime.memmove(SB)
mux.go:237   MOVQ 0x48(SP), R9
mux.go:237   MOVQ 0x90(SP), DX
mux.go:237   MOVQ R9, 0x8(DX)
mux.go:237   MOVQ 0x50(SP), R10
mux.go:237   MOVQ R10, 0x10(DX)
mux.go:237   CMPL runtime.writeBarrier(SB), $0x0
mux.go:237   JNE 
mux.go:237   MOVQ 0x68(SP), R8
mux.go:237   JMP 
mux.go:237   MOVQ 0(DX), R11
mux.go:237   MOVQ R11, AX
mux.go:237   CALL runtime.gcWriteBarrier2(SB)
mux.go:237   MOVQ 0x68(SP), R8
mux.go:237   MOVQ R8, 0(R11)
mux.go:237   MOVQ AX, 0x8(R11)
mux.go:237   MOVQ R8, 0(DX)
mux.go:238   MOVQ 0xf0(SP), DX
mux.go:238   MOVQ 0(DX), DX
mux.go:238   MOVQ 0x8(DX), AX
mux.go:238   MOVQ 0(DX), DX
mux.go:238   MOVQ 0x38(DX), DX
mux.go:238   MOVQ 0xf8(SP), BX
mux.go:238   MOVQ 0x100(SP), CX
mux.go:238   MOVQ 0x108(SP), DI
mux.go:238   MOVQ 0x110(SP), SI
mux.go:238   CALL DX
mux.go:238   MOVQ AX, 0x58(SP)
mux.go:238   MOVQ BX, 0x88(SP)
mux.go:239   MOVQ 0xf0(SP), DX
mux.go:239   MOVQ 0(DX), AX
mux.go:239   TESTB AL, 0(AX)
mux.go:239   ADDQ $0x78, AX
mux.go:239   LEAQ (IP), BX
mux.go:239   MOVQ 0x90(SP), CX
mux.go:239   CALL sync.(*Pool).Put(SB)
mux.go:240   MOVQ 0x58(SP), DX
mux.go:240   MOVQ DX, 0x70(SP)
mux.go:240   MOVQ 0x88(SP), DX
mux.go:240   MOVQ DX, 0x78(SP)
mux.go:240   MOVZX 0x46(SP), DX
mux.go:240   TESTL $0x2, DL
mux.go:240   JE 
mux.go:240   ANDL $-0x3, DX
mux.go:240   MOVB DL, 0x46(SP)
mux.go:240   MOVB DL, 0x47(SP)
mux.go:240   MOVQ 0xd8(SP), DX
mux.go:240   MOVQ 0(DX), AX
mux.go:240   CALL AX
mux.go:240   MOVZX 0x46(SP), DX
mux.go:240   NOPL 0(AX)
mux.go:240   TESTL $0x1, DL
mux.go:240   JE 
mux.go:240   ANDL $-0x2, DX
mux.go:240   MOVB DL, 0x47(SP)
mux.go:240   MOVQ 0xd0(SP), DX
mux.go:240   MOVQ 0(DX), AX
mux.go:240   CALL AX
mux.go:240   MOVQ 0x70(SP), AX
mux.go:240   MOVQ 0x78(SP), BX
mux.go:240   ADDQ $0xe0, SP
mux.go:240   POPQ BP
mux.go:240   RET
mux.go:231   MOVQ AX, 0x70(SP)
mux.go:231   MOVQ BX, 0x78(SP)
mux.go:231   MOVB $0x0, 0x47(SP)
mux.go:231   MOVQ 0xd0(SP), DX
mux.go:231   MOVQ 0(DX), AX
mux.go:231   CALL AX
mux.go:231   MOVQ 0x70(SP), AX
mux.go:231   MOVQ 0x78(SP), BX
mux.go:231   ADDQ $0xe0, SP
mux.go:231   POPQ BP
mux.go:231   RET
mux.go:231   NOPL 0(AX)(AX*1)
mux.go:224   CALL github.com/oioio-space/inkwell/internal/transport/mux.(*sub).sendFragmented(SB)
mux.go:224   MOVQ AX, 0x70(SP)
mux.go:224   MOVQ BX, 0x78(SP)
mux.go:224   ADDQ $0xe0, SP
mux.go:224   POPQ BP
mux.go:224   RET
mux.go:221   MOVQ github.com/oioio-space/inkwell/internal/transport.ErrFrameTooLarge(SB), AX
mux.go:221   MOVQ github.com/oioio-space/inkwell/internal/transport.ErrFrameTooLarge+8(SB), BX
mux.go:221   MOVQ AX, 0x70(SP)
mux.go:221   MOVQ BX, 0x78(SP)
mux.go:221   ADDQ $0xe0, SP
mux.go:221   POPQ BP
mux.go:221   RET
mux.go:221   CALL runtime.deferreturn(SB)
mux.go:221   MOVQ 0x70(SP), AX
mux.go:221   MOVQ 0x78(SP), BX
mux.go:221   ADDQ $0xe0, SP
mux.go:221   POPQ BP
mux.go:221   RET
mux.go:219   MOVQ AX, 0x8(SP)
mux.go:219   MOVQ BX, 0x10(SP)
mux.go:219   MOVQ CX, 0x18(SP)
mux.go:219   MOVQ DI, 0x20(SP)
mux.go:219   MOVQ SI, 0x28(SP)
mux.go:219   MOVQ R8, 0x30(SP)
mux.go:219   MOVQ R9, 0x38(SP)
mux.go:219   MOVQ R10, 0x40(SP)
mux.go:219   CALL runtime.morestack_noctxt.abi0(SB)
mux.go:219   MOVQ 0x8(SP), AX
mux.go:219   MOVQ 0x10(SP), BX
mux.go:219   MOVQ 0x18(SP), CX
mux.go:219   MOVQ 0x20(SP), DI
mux.go:219   MOVQ 0x28(SP), SI
mux.go:219   MOVQ 0x30(SP), R8
mux.go:219   MOVQ 0x38(SP), R9
mux.go:219   MOVQ 0x40(SP), R10
mux.go:219   JMP github.com/oioio-space/inkwell/internal/transport/mux.(*sub).Send(SB)
