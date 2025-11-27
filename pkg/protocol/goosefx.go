package protocol

import (
	"context"
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"soltrading/pkg"
	"soltrading/pkg/pool/goosefx"
	"soltrading/pkg/sol"
)

type GooseFXProtocol struct {
	SolClient *sol.Client
}

func NewGooseFX(solClient *sol.Client) *GooseFXProtocol {
	return &GooseFXProtocol{
		SolClient: solClient,
	}
}

func (p *GooseFXProtocol) ProtocolName() pkg.ProtocolName {
	return pkg.ProtocolName("goosefx")
}

func (p *GooseFXProtocol) FetchPoolsByPair(ctx context.Context, baseMint string, quoteMint string) ([]pkg.Pool, error) {
	baseMintPubkey, err := solana.PublicKeyFromBase58(baseMint)
	if err != nil {
		return nil, fmt.Errorf("invalid base mint address: %w", err)
	}
	quoteMintPubkey, err := solana.PublicKeyFromBase58(quoteMint)
	if err != nil {
		return nil, fmt.Errorf("invalid quote mint address: %w", err)
	}

	// Fetch pools with TokenMintA = baseMint and TokenMintB = quoteMint
	filters := []rpc.RPCFilter{
		{
			Memcmp: &rpc.RPCFilterMemcmp{
				Offset: 8, // After discriminator
				Bytes:  baseMintPubkey.Bytes(),
			},
		},
		{
			Memcmp: &rpc.RPCFilterMemcmp{
				Offset: 40, // After TokenMintA
				Bytes:  quoteMintPubkey.Bytes(),
			},
		},
	}

	programAccounts, err := p.SolClient.GetProgramAccountsWithOpts(ctx, goosefx.GooseFXProgramID, &rpc.GetProgramAccountsOpts{
		Filters: filters,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch GooseFX pools: %w", err)
	}

	// Also try reverse pair
	filtersReverse := []rpc.RPCFilter{
		{
			Memcmp: &rpc.RPCFilterMemcmp{
				Offset: 8,
				Bytes:  quoteMintPubkey.Bytes(),
			},
		},
		{
			Memcmp: &rpc.RPCFilterMemcmp{
				Offset: 40,
				Bytes:  baseMintPubkey.Bytes(),
			},
		},
	}

	reverseAccounts, err := p.SolClient.GetProgramAccountsWithOpts(ctx, goosefx.GooseFXProgramID, &rpc.GetProgramAccountsOpts{
		Filters: filtersReverse,
	})
	if err == nil {
		programAccounts = append(programAccounts, reverseAccounts...)
	}

	res := make([]pkg.Pool, 0)
	for _, v := range programAccounts {
		pool := &goosefx.GooseFXPool{}
		if err := pool.Decode(v.Account.Data.GetBinary()); err != nil {
			continue
		}
		pool.PoolId = v.Pubkey
		res = append(res, pool)
	}
	return res, nil
}

func (p *GooseFXProtocol) FetchPoolByID(ctx context.Context, poolId string) (pkg.Pool, error) {
	poolPubkey, err := solana.PublicKeyFromBase58(poolId)
	if err != nil {
		return nil, fmt.Errorf("invalid pool ID: %w", err)
	}

	account, err := p.SolClient.GetAccountInfoWithOpts(ctx, poolPubkey)
	if err != nil {
		return nil, fmt.Errorf("failed to get pool account %s: %w", poolId, err)
	}

	pool := &goosefx.GooseFXPool{}
	if err := pool.Decode(account.Value.Data.GetBinary()); err != nil {
		return nil, fmt.Errorf("failed to parse pool data for pool %s: %w", poolId, err)
	}
	pool.PoolId = poolPubkey
	return pool, nil
}
